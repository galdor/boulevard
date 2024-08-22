package httpserver

import (
	"encoding/json"
	"fmt"
	"net/http"

	"go.n16f.net/ejson"
)

type MatchSpec struct {
	Method      string       `json:"method,omitempty"`
	Path        string       `json:"path,omitempty"`
	PathPattern *PathPattern `json:"-"`
}

func (s *MatchSpec) MarshalJSON() ([]byte, error) {
	type MatchSpec2 MatchSpec
	s2 := MatchSpec2(*s)

	if s2.PathPattern != nil {
		s2.Path = s2.PathPattern.String()
	}

	return json.Marshal(s2)
}

func (s *MatchSpec) UnmarshalJSON(data []byte) error {
	type MatchSpec2 MatchSpec
	s2 := MatchSpec2(*s)

	if err := json.Unmarshal(data, &s2); err != nil {
		return err
	}

	if s2.Path != "" {
		var pp PathPattern

		if err := pp.Parse(s2.Path); err != nil {
			return fmt.Errorf("cannot parse path pattern: %w", err)
		}

		s2.PathPattern = &pp
	}

	*s = MatchSpec(s2)
	return nil
}

type Handler struct {
	Match MatchSpec `json:"match"`

	Reply *ReplyAction `json:"reply,omitempty"`
	Serve *ServeAction `json:"serve,omitempty"`
	Proxy *ProxyAction `json:"proxy,omitempty"`
}

func (h *Handler) ValidateJSON(v *ejson.Validator) {
	v.CheckObject("match", &h.Match)

	nbActions := 0
	if h.Serve != nil {
		nbActions++
	}
	if h.Reply != nil {
		nbActions++
	}
	if h.Proxy != nil {
		nbActions++
	}

	if nbActions == 0 {
		v.AddError(nil, "missing_action", "handler must contain an action")
	} else if nbActions > 1 {
		v.AddError(nil, "multiple_actions",
			"handler must contain a single action")
	}

	v.CheckOptionalObject("serve", h.Serve)
	v.CheckOptionalObject("reply", h.Reply)
	v.CheckOptionalObject("proxy", h.Proxy)
}

func (mod *Module) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	subpath := req.URL.Path
	if len(subpath) > 0 && subpath[0] == '/' {
		subpath = subpath[1:]
	}

	ctx := RequestContext{
		Log: mod.Log,

		Request:        req,
		ResponseWriter: w,

		Subpath: subpath,
	}

	h := mod.findHandler(&ctx)
	if h == nil {
		w.WriteHeader(404)
		return
	}

	var fn func(*Handler, *RequestContext)

	switch {
	case h.Reply != nil:
		fn = mod.reply
	case h.Serve != nil:
		fn = mod.serve
	case h.Proxy != nil:
		fn = mod.proxy
	}

	fn(h, &ctx)
}

func (mod *Module) findHandler(ctx *RequestContext) *Handler {
	for _, h := range mod.Cfg.Handlers {
		if h.matchRequest(ctx) {
			return h
		}
	}
	return nil
}

func (h *Handler) matchRequest(ctx *RequestContext) bool {
	matchSpec := h.Match
	if matchSpec.Method != "" && matchSpec.Method != ctx.Request.Method {
		return false
	}

	if pattern := matchSpec.PathPattern; pattern != nil {
		match, subpath := pattern.Match(ctx.Request.URL.Path)
		if !match {
			return false
		}

		ctx.Subpath = subpath
	}

	return true
}

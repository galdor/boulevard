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

type ReplyAction struct {
	Status int    `json:"status"`
	Reason string `json:"reason,omitempty"`
	Body   string `json:"body,omitempty"`
}

func (action *ReplyAction) ValidateJSON(v *ejson.Validator) {
	v.CheckIntMinMax("status", action.Status, 200, 599)
}

type ServeAction struct {
	Path string `json:"path"`
}

func (action *ServeAction) ValidateJSON(v *ejson.Validator) {
	v.CheckStringNotEmpty("path", action.Path)
}

type ProxyAction struct {
	// TODO
}

func (action *ProxyAction) ValidateJSON(v *ejson.Validator) {
	// TODO
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
	h := mod.findHandler(req)
	if h == nil {
		w.WriteHeader(404)
		return
	}

	var fn func(*Handler, http.ResponseWriter, *http.Request)

	switch {
	case h.Reply != nil:
		fn = mod.reply
	case h.Serve != nil:
		fn = mod.serve
	case h.Proxy != nil:
		fn = mod.proxy
	}

	fn(h, w, req)
}

func (mod *Module) findHandler(req *http.Request) *Handler {
	for _, h := range mod.Cfg.Handlers {
		if h.matchRequest(req) {
			return h
		}
	}
	return nil
}

func (h *Handler) matchRequest(req *http.Request) bool {
	match := h.Match
	if match.Method != "" && match.Method != req.Method {
		return false
	}

	if match.PathPattern != nil && !match.PathPattern.Match(req.URL.Path) {
		return false
	}

	return true
}

func (mod *Module) reply(h *Handler, w http.ResponseWriter, req *http.Request) {
	action := h.Reply

	w.WriteHeader(action.Status)
	w.Write([]byte(action.Body))
}

func (mod *Module) serve(h *Handler, w http.ResponseWriter, req *http.Request) {
	// TODO
	w.WriteHeader(501)
	fmt.Fprintf(w, "serve action not implemented")
}

func (mod *Module) proxy(h *Handler, w http.ResponseWriter, req *http.Request) {
	// TODO
	w.WriteHeader(501)
	fmt.Fprintf(w, "proxy action not implemented")
}

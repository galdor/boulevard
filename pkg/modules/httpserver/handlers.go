package httpserver

import (
	"encoding/json"
	"fmt"
	"net/http"

	"go.n16f.net/ejson"
)

type Handler struct {
	Method      string       `json:"method,omitempty"`
	Path        string       `json:"path,omitempty"`
	PathPattern *PathPattern `json:"-"`

	Reply *ReplyAction `json:"reply,omitempty"`
	Serve *ServeAction `json:"serve,omitempty"`
	Proxy *ProxyAction `json:"proxy,omitempty"`
}

func (h *Handler) MarshalJSON() ([]byte, error) {
	type Handler2 Handler
	h2 := Handler2(*h)

	if h2.PathPattern != nil {
		h2.Path = h2.PathPattern.String()
	}

	return json.Marshal(h2)
}

func (h *Handler) UnmarshalJSON(data []byte) error {
	type Handler2 Handler
	h2 := Handler2(*h)

	if err := json.Unmarshal(data, &h2); err != nil {
		return err
	}

	if h2.Path != "" {
		var pp PathPattern

		if err := pp.Parse(h2.Path); err != nil {
			return fmt.Errorf("cannot parse path pattern: %w", err)
		}

		h2.PathPattern = &pp
	}

	*h = Handler(h2)
	return nil
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
	if h.Method != "" && h.Method != req.Method {
		return false
	}

	if h.PathPattern != nil && !h.PathPattern.Match(req.URL.Path) {
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

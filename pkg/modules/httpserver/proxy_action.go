package httpserver

import (
	"fmt"

	"go.n16f.net/ejson"
)

type ProxyAction struct {
	// TODO
}

func (action *ProxyAction) ValidateJSON(v *ejson.Validator) {
	// TODO
}

func (mod *Module) proxy(h *Handler, ctx *RequestContext) {
	w := ctx.ResponseWriter

	// TODO
	w.WriteHeader(501)
	fmt.Fprintf(w, "proxy action not implemented")
}

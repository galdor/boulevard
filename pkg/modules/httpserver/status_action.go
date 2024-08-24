package httpserver

import (
	"fmt"

	"go.n16f.net/ejson"
)

type StatusAction struct {
	// TODO
}

func (action *StatusAction) ValidateJSON(v *ejson.Validator) {
	// TODO
}

func (mod *Module) status(h *Handler, ctx *RequestContext) {
	w := ctx.ResponseWriter

	// TODO
	w.WriteHeader(501)
	fmt.Fprintf(w, "status action not implemented")
}

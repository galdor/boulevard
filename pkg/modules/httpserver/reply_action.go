package httpserver

import (
	"strings"

	"go.n16f.net/ejson"
)

type ReplyAction struct {
	Status int    `json:"status"`
	Reason string `json:"reason,omitempty"`
	Body   string `json:"body,omitempty"`
}

func (action *ReplyAction) ValidateJSON(v *ejson.Validator) {
	v.CheckIntMinMax("status", action.Status, 200, 599)
}

func (mod *Module) reply(h *Handler, ctx *RequestContext) {
	ctx.Reply(h.Reply.Status, strings.NewReader(h.Reply.Body))
}

package httpserver

import (
	"strings"

	"go.n16f.net/ejson"
)

type ReplyActionCfg struct {
	Status int    `json:"status"`
	Reason string `json:"reason,omitempty"`
	Header Header `json:"header,omitempty"`
	Body   string `json:"body,omitempty"`
}

func (cfg *ReplyActionCfg) ValidateJSON(v *ejson.Validator) {
	v.CheckIntMinMax("status", cfg.Status, 200, 599)
}

type ReplyAction struct {
	Handler *Handler
	Cfg     ReplyActionCfg
}

func NewReplyAction(h *Handler, cfg ReplyActionCfg) (*ReplyAction, error) {
	a := ReplyAction{
		Handler: h,
		Cfg:     cfg,
	}

	return &a, nil
}

func (a *ReplyAction) Start() error {
	return nil
}

func (a *ReplyAction) Stop() {
}

func (a *ReplyAction) HandleRequest(ctx *RequestContext) {
	a.Cfg.Header.Apply(ctx.ResponseWriter.Header())
	ctx.Reply(a.Cfg.Status, strings.NewReader(a.Cfg.Body))
}

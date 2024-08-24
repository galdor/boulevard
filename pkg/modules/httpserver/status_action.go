package httpserver

import (
	"go.n16f.net/ejson"
)

type StatusActionCfg struct {
	// TODO
}

func (cfg *StatusActionCfg) ValidateJSON(v *ejson.Validator) {
	// TODO
}

type StatusAction struct {
	Handler *Handler
	Cfg     StatusActionCfg
}

func NewStatusAction(h *Handler, cfg StatusActionCfg) (*StatusAction, error) {
	a := StatusAction{
		Handler: h,
		Cfg:     cfg,
	}

	return &a, nil
}

func (a *StatusAction) Start() error {
	return nil
}

func (a *StatusAction) Stop() {
}

func (a *StatusAction) HandleRequest(ctx *RequestContext) {
	// TODO
	ctx.ReplyError(501)
}

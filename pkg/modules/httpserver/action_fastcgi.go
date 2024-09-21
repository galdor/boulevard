package httpserver

import (
	"go.n16f.net/ejson"
)

type FastCGIActionCfg struct {
	Address string `json:"address"`
}

func (cfg *FastCGIActionCfg) ValidateJSON(v *ejson.Validator) {
	v.CheckNetworkAddress("address", cfg.Address)

}

type FastCGIAction struct {
	Handler *Handler
	Cfg     FastCGIActionCfg
}

func NewFastCGIAction(h *Handler, cfg FastCGIActionCfg) (*FastCGIAction, error) {
	a := FastCGIAction{
		Handler: h,
		Cfg:     cfg,
	}

	return &a, nil
}

func (a *FastCGIAction) Start() error {
	return nil
}

func (a *FastCGIAction) Stop() {
}

func (a *FastCGIAction) HandleRequest(ctx *RequestContext) {
	// TODO
	ctx.ReplyError(501)
}

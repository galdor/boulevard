package httpserver

import (
	"go.n16f.net/ejson"
)

type ProxyActionCfg struct {
	// TODO
}

func (cfg *ProxyActionCfg) ValidateJSON(v *ejson.Validator) {
	// TODO
}

type ProxyAction struct {
	Handler *Handler
	Cfg     ProxyActionCfg
}

func NewProxyAction(h *Handler, cfg ProxyActionCfg) (*ProxyAction, error) {
	a := ProxyAction{
		Handler: h,
		Cfg:     cfg,
	}

	return &a, nil
}

func (a *ProxyAction) Start() error {
	return nil
}

func (a *ProxyAction) Stop() {
}

func (a *ProxyAction) HandleRequest(ctx *RequestContext) {
	// TODO
	ctx.ReplyError(501)
}

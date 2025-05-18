package http

import (
	"fmt"

	"go.n16f.net/bcl"
	"go.n16f.net/boulevard/pkg/boulevard"
	"go.n16f.net/boulevard/pkg/httputils"
)

type RedirectActionCfg struct {
	Status int
	URI    *boulevard.FormatString
	Header HeaderOps
	Body   *boulevard.FormatString
}

func (cfg *RedirectActionCfg) ReadBCLElement(elt *bcl.Element) error {
	cfg.Status = 302

	if elt.IsBlock() {
		elt.MaybeEntryValues("status", bcl.WithValueValidation(&cfg.Status,
			httputils.ValidateBCLStatus))
		elt.EntryValues("uri", &cfg.URI)
		elt.MaybeBlock("header", &cfg.Header)
		elt.MaybeEntryValues("body", &cfg.Body)
	} else {
		elt.Values(bcl.WithValueValidation(&cfg.Status,
			httputils.ValidateBCLStatus),
			&cfg.URI)
	}

	return nil
}

type RedirectAction struct {
	Handler *Handler
	Cfg     *RedirectActionCfg

	view *View
}

func NewRedirectAction(h *Handler, cfg *RedirectActionCfg) (*RedirectAction, error) {
	view, err := NewView("templates/redirect")
	if err != nil {
		return nil, fmt.Errorf("cannot create view: %w", err)
	}

	a := RedirectAction{
		Handler: h,
		Cfg:     cfg,

		view: view,
	}

	return &a, nil
}

func (a *RedirectAction) Start() error {
	return nil
}

func (a *RedirectAction) Stop() {
}

func (a *RedirectAction) HandleRequest(ctx *RequestContext) {
	uriString := a.Cfg.URI.Expand(ctx.Vars)
	redirect(a.Cfg.Status, uriString, a.Cfg.Header, a.Cfg.Body, a.view, ctx)
}

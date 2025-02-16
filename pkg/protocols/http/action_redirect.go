package http

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

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
		elt.MaybeEntryValue("status", bcl.WithValueValidation(&cfg.Status,
			httputils.ValidateBCLStatus))
		elt.EntryValue("uri", &cfg.URI)
		elt.MaybeBlock("header", &cfg.Header)
		elt.MaybeEntryValue("body", &cfg.Body)
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
	uri, err := url.Parse(uriString)
	if err != nil {
		ctx.Log.Error("cannot parse redirection URI %q: %v", uriString, err)
		ctx.ReplyError(500)
		return
	}

	ctx.Vars["http.redirect.uri"] = uri.String()

	status := 302
	if a.Cfg.Status != 0 {
		status = a.Cfg.Status
	}

	header := ctx.ResponseWriter.Header()
	header.Set("Location", uri.String())
	a.Cfg.Header.Apply(header, ctx.Vars)

	var body io.Reader
	if ctx.Request.Method == "GET" {
		if a.Cfg.Body == nil {
			header.Set("Content-Type", MediaTypeHTML.String())

			tplData := struct {
				Status int    `json:"status"`
				Reason string `json:"reason"`
				URI    string `json:"uri"`
			}{
				Status: status,
				Reason: http.StatusText(status),
				URI:    uri.String(),
			}

			bodyData, err := a.view.Render("redirect", tplData, ctx)
			if err != nil {
				ctx.Log.Error("cannot render redirection data: %v", err)
				ctx.ReplyError(500)
				return
			}

			body = bytes.NewReader(bodyData)
		} else {
			bodyString := a.Cfg.Body.Expand(ctx.Vars)
			body = strings.NewReader(bodyString)
		}
	}

	ctx.Reply(status, body)
}

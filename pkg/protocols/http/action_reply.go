package http

import (
	"io"
	"strings"

	"go.n16f.net/bcl"
	"go.n16f.net/boulevard/pkg/boulevard"
	"go.n16f.net/boulevard/pkg/httputils"
)

type ReplyActionCfg struct {
	Status int
	Header HeaderOps
	Body   *boulevard.FormatString
}

func (cfg *ReplyActionCfg) ReadBCLElement(elt *bcl.Element) error {
	cfg.Status = 200

	if elt.IsBlock() {
		elt.MaybeEntryValues("status",
			bcl.WithValueValidation(&cfg.Status, httputils.ValidateBCLStatus))
		elt.MaybeBlock("header", &cfg.Header)
		elt.MaybeEntryValues("body", &cfg.Body)
	} else {
		if elt.CheckMinMaxNbValues(1, 2) {
			switch elt.NbValues() {
			case 1:
				elt.Values(
					bcl.WithValueValidation(&cfg.Status,
						httputils.ValidateBCLStatus))
			case 2:
				elt.Values(
					bcl.WithValueValidation(&cfg.Status,
						httputils.ValidateBCLStatus),
					&cfg.Body)
				elt.Values(&cfg.Status, &cfg.Body)
			}
		}
	}

	return nil
}

type ReplyAction struct {
	Handler *Handler
	Cfg     *ReplyActionCfg
}

func NewReplyAction(h *Handler, cfg *ReplyActionCfg) (*ReplyAction, error) {
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
	status := 200
	if a.Cfg.Status != 0 {
		status = a.Cfg.Status
	}

	a.Cfg.Header.Apply(ctx.ResponseWriter.Header(), ctx.Vars)

	var body io.Reader
	if a.Cfg.Body != nil {
		bodyString := a.Cfg.Body.Expand(ctx.Vars)
		body = strings.NewReader(bodyString)
	}

	ctx.Reply(status, body)
}

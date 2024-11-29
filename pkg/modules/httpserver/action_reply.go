package httpserver

import (
	"io"
	"strings"

	"go.n16f.net/boulevard/pkg/boulevard"
	"go.n16f.net/ejson"
)

type ReplyActionCfg struct {
	Status int                     `json:"status,omitempty"`
	Reason string                  `json:"reason,omitempty"`
	Header Header                  `json:"header,omitempty"`
	Body   *boulevard.FormatString `json:"body,omitempty"`
}

func (cfg *ReplyActionCfg) ValidateJSON(v *ejson.Validator) {
	if cfg.Status != 0 {
		v.CheckIntMinMax("status", cfg.Status, 200, 599)
	}

	v.CheckObjectMap("header", cfg.Header)
	v.CheckOptionalObject("body", cfg.Body)
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

package httpserver

import (
	"io"
	"strings"

	"go.n16f.net/bcl"
	"go.n16f.net/boulevard/pkg/boulevard"
)

type ReplyActionCfg struct {
	Status int
	Header Header
	Body   *boulevard.FormatString
}

func (cfg *ReplyActionCfg) Init(elt *bcl.Element) {
	cfg.Status = 200

	if elt.IsBlock() {
		// TODO Validate integer 200..599
		elt.MaybeEntryValue("status", &cfg.Status)

		cfg.Header = make(Header)
		for _, entry := range elt.Entries("header") {
			var name string
			var value boulevard.FormatString

			if entry.Values(&name, &value) {
				cfg.Header[name] = &value
			}
		}

		elt.MaybeEntryValue("body", &cfg.Body)
	} else {
		if elt.CheckMinMaxValues(1, 2) {
			switch elt.NbValues() {
			case 1:
				// TODO Validate integer 200..599
				elt.Values(&cfg.Status)
			case 2:
				// TODO Validate integer 200..599
				elt.Values(&cfg.Status, &cfg.Body)
			}
		}
	}
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

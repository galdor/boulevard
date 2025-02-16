package http

import (
	"bytes"
	"fmt"
	"maps"
	"slices"

	"go.n16f.net/bcl"
	"go.n16f.net/boulevard/pkg/boulevard"
)

var statusMediaTypes = []*MediaType{
	MediaTypeText,
	MediaTypeHTML,
	MediaTypeJSON,
}

type StatusData struct {
	Servers []*boulevard.ServerStatus `json:"servers"`
}

type StatusActionCfg struct {
}

func (cfg *StatusActionCfg) ReadBCLElement(elt *bcl.Element) error {
	if elt.IsBlock() {
	} else {
	}

	return nil
}

type StatusAction struct {
	Handler *Handler
	Cfg     *StatusActionCfg

	view *View
}

func NewStatusAction(h *Handler, cfg *StatusActionCfg) (*StatusAction, error) {
	view, err := NewView("templates/status")
	if err != nil {
		return nil, fmt.Errorf("cannot create view: %w", err)
	}

	a := StatusAction{
		Handler: h,
		Cfg:     cfg,

		view: view,
	}

	return &a, nil
}

func (a *StatusAction) Start() error {
	return nil
}

func (a *StatusAction) Stop() {
}

func (a *StatusAction) HandleRequest(ctx *RequestContext) {
	statusTable := ctx.Protocol.Server.Cfg.ServerStatuses()

	statuses := make([]*boulevard.ServerStatus, 0, len(statusTable))
	for _, name := range slices.Sorted(maps.Keys(statusTable)) {
		statuses = append(statuses, statusTable[name])
	}

	status := StatusData{
		Servers: statuses,
	}

	content, err := a.view.Render("status", &status, ctx)
	if err != nil {
		ctx.Log.Error("cannot render status data: %v", err)
		ctx.ReplyError(500)
		return
	}

	ctx.Reply(200, bytes.NewReader(content))
}

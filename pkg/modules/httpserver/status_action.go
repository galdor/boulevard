package httpserver

import (
	"bytes"
	"embed"
	"fmt"
	htmltemplate "html/template"
	"path"
	"slices"
	"strings"

	"go.n16f.net/boulevard/pkg/boulevard"
	"go.n16f.net/ejson"
)

//go:embed templates/**/*
var htmlTemplateFS embed.FS

type StatusActionCfg struct {
	// TODO
}

func (cfg *StatusActionCfg) ValidateJSON(v *ejson.Validator) {
	// TODO
}

type StatusAction struct {
	Handler *Handler
	Cfg     StatusActionCfg

	htmlTemplates *htmltemplate.Template
}

func NewStatusAction(h *Handler, cfg StatusActionCfg) (*StatusAction, error) {
	a := StatusAction{
		Handler: h,
		Cfg:     cfg,
	}

	return &a, nil
}

func (a *StatusAction) Start() error {
	htmlTemplates, err := boulevard.LoadHTMLTemplates(htmlTemplateFS,
		"templates/status/html/*.gotpl")
	if err != nil {
		return fmt.Errorf("cannot load HTML templates: %w", err)
	}
	a.htmlTemplates = htmlTemplates

	return nil
}

func (a *StatusAction) Stop() {
}

func (a *StatusAction) HandleRequest(ctx *RequestContext) {
	modData := a.Handler.Module.Data

	modStatuses := modData.ModuleStatuses()
	slices.SortFunc(modStatuses,
		func(ms1, ms2 *boulevard.ModuleStatus) int {
			return strings.Compare(ms1.Name, ms2.Name)
		})

	tplData := struct {
		Modules    []*boulevard.ModuleStatus
		ModuleData []htmltemplate.HTML
	}{
		Modules:    modStatuses,
		ModuleData: make([]htmltemplate.HTML, len(modStatuses)),
	}

	for i, mod := range modStatuses {
		content, err := a.renderHTMLTemplate(ctx, mod.Info.Name, mod.Data)
		if err != nil {
			ctx.ReplyError(500)
			return
		}

		tplData.ModuleData[i] = htmltemplate.HTML(content)
	}

	content, err := a.renderHTMLTemplate(ctx, "status", tplData)
	if err != nil {
		ctx.ReplyError(500)
		return
	}

	ctx.Reply(200, bytes.NewReader(content))
}

func (a *StatusAction) renderHTMLTemplate(ctx *RequestContext, name string, data any) ([]byte, error) {
	name = path.Join("templates/status/html", name)

	var buf bytes.Buffer
	if err := a.htmlTemplates.ExecuteTemplate(&buf, name, data); err != nil {
		ctx.Log.Error("cannot render template %q: %v", name, err)
		return nil, err
	}

	return buf.Bytes(), nil
}

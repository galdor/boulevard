package http

import (
	"bytes"
	"encoding/json"
	"fmt"
	htmltemplate "html/template"
	"maps"
	"path"
	"slices"
	texttemplate "text/template"

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

	textTemplates *texttemplate.Template
	htmlTemplates *htmltemplate.Template
}

func NewStatusAction(h *Handler, cfg *StatusActionCfg) (*StatusAction, error) {
	a := StatusAction{
		Handler: h,
		Cfg:     cfg,
	}

	return &a, nil
}

func (a *StatusAction) Start() error {
	textTemplates, err := boulevard.LoadTextTemplates(htmlTemplateFS,
		"templates/status/text/*.gotpl")
	if err != nil {
		return fmt.Errorf("cannot load text templates: %w", err)
	}
	a.textTemplates = textTemplates

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
	statusTable := ctx.Protocol.Server.Cfg.ServerStatuses()

	statuses := make([]*boulevard.ServerStatus, 0, len(statusTable))
	for _, name := range slices.Sorted(maps.Keys(statusTable)) {
		statuses = append(statuses, statusTable[name])
	}

	status := StatusData{
		Servers: statuses,
	}

	var fn func(*RequestContext, *StatusData) ([]byte, error)

	switch ctx.NegotiateMediaType(statusMediaTypes) {
	case MediaTypeText:
		fn = a.textContent
	case MediaTypeHTML:
		fn = a.htmlContent
	case MediaTypeJSON:
		fn = a.jsonContent
	}

	content, err := fn(ctx, &status)
	if err != nil {
		ctx.Log.Error("cannot render status data: %v", err)
		ctx.ReplyError(500)
		return
	}

	ctx.Reply(200, bytes.NewReader(content))
}

func (a *StatusAction) textContent(ctx *RequestContext, status *StatusData) ([]byte, error) {
	tplData := struct {
		Servers    []*boulevard.ServerStatus
		ServerData []string
	}{
		Servers:    status.Servers,
		ServerData: make([]string, len(status.Servers)),
	}

	for i, server := range status.Servers {
		tplName := server.Protocol

		content, err := a.renderTextTemplate(ctx, tplName, server.Data)
		if err != nil {
			return nil, fmt.Errorf("cannot render template %q: %w",
				tplName, err)
		}

		tplData.ServerData[i] = string(content)
	}

	content, err := a.renderTextTemplate(ctx, "status", tplData)
	if err != nil {
		return nil, fmt.Errorf("cannot render template %q: %w",
			"status", err)
	}

	return content, nil
}

func (a *StatusAction) htmlContent(ctx *RequestContext, status *StatusData) ([]byte, error) {
	tplData := struct {
		Servers    []*boulevard.ServerStatus
		ServerData []htmltemplate.HTML
	}{
		Servers:    status.Servers,
		ServerData: make([]htmltemplate.HTML, len(status.Servers)),
	}

	for i, server := range status.Servers {
		tplName := server.Protocol

		content, err := a.renderHTMLTemplate(ctx, tplName, server.Data)
		if err != nil {
			return nil, fmt.Errorf("cannot render template %q: %w",
				tplName, err)
		}

		tplData.ServerData[i] = htmltemplate.HTML(content)
	}

	content, err := a.renderHTMLTemplate(ctx, "status", tplData)
	if err != nil {
		return nil, fmt.Errorf("cannot render template %q: %w",
			"status", err)
	}

	return content, nil
}

func (a *StatusAction) jsonContent(ctx *RequestContext, status *StatusData) ([]byte, error) {
	return json.MarshalIndent(status, "", "  ")
}

func (a *StatusAction) renderTextTemplate(ctx *RequestContext, name string, data any) ([]byte, error) {
	name = path.Join("templates/status/text", name)

	var buf bytes.Buffer
	if err := a.textTemplates.ExecuteTemplate(&buf, name, data); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (a *StatusAction) renderHTMLTemplate(ctx *RequestContext, name string, data any) ([]byte, error) {
	name = path.Join("templates/status/html", name)

	var buf bytes.Buffer
	if err := a.htmlTemplates.ExecuteTemplate(&buf, name, data); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

package http

import (
	"bytes"
	"encoding/json"
	"fmt"
	htmltemplate "html/template"
	"path"
	texttemplate "text/template"

	"go.n16f.net/boulevard/pkg/boulevard"
)

type View struct {
	TemplateDirectory string

	textTemplates *texttemplate.Template
	htmlTemplates *htmltemplate.Template
}

func NewView(tplDir string) (*View, error) {
	v := View{
		TemplateDirectory: tplDir,
	}

	if err := v.Load(); err != nil {
		return nil, err
	}

	return &v, nil
}

func (v *View) Load() error {
	textTemplates, err := boulevard.LoadTextTemplates(templateFS,
		path.Join(v.TemplateDirectory, "text", "*.gotpl"))
	if err != nil {
		return fmt.Errorf("cannot load text templates: %w", err)
	}

	htmlTemplates, err := boulevard.LoadHTMLTemplates(templateFS,
		path.Join(v.TemplateDirectory, "html", "*.gotpl"))
	if err != nil {
		return fmt.Errorf("cannot load HTML templates: %w", err)
	}

	v.textTemplates = textTemplates
	v.htmlTemplates = htmlTemplates

	return nil
}

func (v *View) Render(tplName string, data any, ctx *RequestContext) ([]byte, error) {
	mediaTypes := []*MediaType{MediaTypeJSON}

	if v.textTemplates != nil {
		mediaTypes = append(mediaTypes, MediaTypeText)
	}

	if v.htmlTemplates != nil {
		mediaTypes = append(mediaTypes, MediaTypeHTML)
	}

	var fn func(string, any, *RequestContext) ([]byte, error)

	switch ctx.NegotiateMediaType(statusMediaTypes) {
	case MediaTypeJSON:
		fn = v.renderJSON
	case MediaTypeText:
		fn = v.renderText
	case MediaTypeHTML:
		fn = v.renderHTML
	}

	return fn(tplName, data, ctx)
}

func (v *View) renderJSON(tplName string, data any, ctx *RequestContext) ([]byte, error) {
	content, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, err
	}

	return append(content, '\n'), nil
}

func (v *View) renderText(tplName string, data any, ctx *RequestContext) ([]byte, error) {
	tplPath := path.Join(v.TemplateDirectory, "text", tplName)

	var buf bytes.Buffer
	if err := v.textTemplates.ExecuteTemplate(&buf, tplPath, data); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (v *View) renderHTML(tplName string, data any, ctx *RequestContext) ([]byte, error) {
	tplPath := path.Join(v.TemplateDirectory, "html", tplName)

	var buf bytes.Buffer
	if err := v.htmlTemplates.ExecuteTemplate(&buf, tplPath, data); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

package httpserver

import (
	"errors"
	"io/fs"
	"net/http"
	"os"
	"path"

	"go.n16f.net/ejson"
)

type ServeActionCfg struct {
	Path string `json:"path"`
}

func (cfg *ServeActionCfg) ValidateJSON(v *ejson.Validator) {
	v.CheckStringNotEmpty("path", cfg.Path)
}

type ServeAction struct {
	Handler *Handler
	Cfg     ServeActionCfg
}

func NewServeAction(h *Handler, cfg ServeActionCfg) (*ServeAction, error) {
	a := ServeAction{
		Handler: h,
		Cfg:     cfg,
	}

	return &a, nil
}

func (a *ServeAction) Start() error {
	return nil
}

func (a *ServeAction) Stop() {
}

func (a *ServeAction) HandleRequest(ctx *RequestContext) {
	filePath := path.Join(a.Cfg.Path, ctx.Subpath)

	info, err := os.Stat(filePath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			ctx.ReplyError(404)
			return
		}

		ctx.Log.Error("cannot stat %q: %v", filePath, err)
		ctx.ReplyError(500)
		return
	}

	if !info.Mode().IsRegular() {
		ctx.ReplyError(403)
		return
	}

	modTime := info.ModTime()

	body, err := os.Open(filePath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			ctx.ReplyError(404)
			return
		}

		ctx.Log.Error("cannot open %q: %v", filePath, err)
		ctx.ReplyError(500)
		return
	}
	defer body.Close()

	http.ServeContent(ctx.ResponseWriter, ctx.Request, filePath, modTime, body)
}
package httpserver

import (
	"errors"
	"io/fs"
	"net/http"
	"os"
	"path"

	"go.n16f.net/ejson"
)

type ServeAction struct {
	Path string `json:"path"`
}

func (action *ServeAction) ValidateJSON(v *ejson.Validator) {
	v.CheckStringNotEmpty("path", action.Path)
}

func (mod *Module) serve(h *Handler, ctx *RequestContext) {
	filePath := path.Join(h.Serve.Path, ctx.Subpath)

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

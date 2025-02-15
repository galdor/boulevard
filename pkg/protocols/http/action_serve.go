package http

import (
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path"

	"go.n16f.net/bcl"
	"go.n16f.net/boulevard/pkg/boulevard"
)

type ServeActionCfg struct {
	Path         *boulevard.FormatString
	IndexFiles   []string
	FileNotFound *ServeActionFileNotFoundCfg
}

func (cfg *ServeActionCfg) ReadBCLElement(elt *bcl.Element) error {
	if elt.IsBlock() {
		elt.EntryValue("path", &cfg.Path)

		for _, entry := range elt.FindEntries("index_file") {
			var file string
			entry.Value(&file)
			cfg.IndexFiles = append(cfg.IndexFiles, file)
		}

		elt.MaybeBlock("file_not_found", &cfg.FileNotFound)
	} else {
		elt.Values(&cfg.Path)
	}

	return nil
}

type ServeActionFileNotFoundCfg struct {
	Reply *ReplyActionCfg
}

func (cfg *ServeActionFileNotFoundCfg) ReadBCLElement(block *bcl.Element) error {
	block.MaybeElement("reply", &cfg.Reply)
	return nil
}

type ServeAction struct {
	Handler           *Handler
	Cfg               *ServeActionCfg
	FileNotFoundReply *ReplyAction
}

func NewServeAction(h *Handler, cfg *ServeActionCfg) (*ServeAction, error) {
	a := ServeAction{
		Handler: h,
		Cfg:     cfg,
	}

	if cfg.FileNotFound != nil && cfg.FileNotFound.Reply != nil {
		reply, err := NewReplyAction(h, cfg.FileNotFound.Reply)
		if err != nil {
			return nil, fmt.Errorf("cannot create file not found reply "+
				"action: %w", err)
		}

		a.FileNotFoundReply = reply
	}

	return &a, nil
}

func (a *ServeAction) Start() error {
	if a.FileNotFoundReply != nil {
		if err := a.FileNotFoundReply.Start(); err != nil {
			return fmt.Errorf("cannot start file not found reply action: %w",
				err)
		}
	}

	return nil
}

func (a *ServeAction) Stop() {
	if a.FileNotFoundReply != nil {
		a.FileNotFoundReply.Stop()
	}
}

func (a *ServeAction) HandleRequest(ctx *RequestContext) {
	basePath := a.Cfg.Path.Expand(ctx.Vars)

	filePath := path.Join(basePath, ctx.Subpath)

	info, err := os.Stat(filePath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			if a.FileNotFoundReply == nil {
				ctx.ReplyError(404)
			} else {
				a.FileNotFoundReply.HandleRequest(ctx)
			}

			return
		}

		ctx.Log.Error("cannot stat %q: %v", filePath, err)
		ctx.ReplyError(500)
		return
	}

	if info.Mode().IsDir() {
		for _, indexFile := range a.Cfg.IndexFiles {
			indexFilePath := path.Join(basePath, indexFile)
			indexInfo, err := os.Stat(indexFilePath)
			if err == nil {
				body, err := os.Open(indexFilePath)
				if err == nil {
					defer body.Close()

					http.ServeContent(ctx.ResponseWriter, ctx.Request,
						indexFilePath, indexInfo.ModTime(), body)
					return
				}
			}
		}

		ctx.ReplyError(403)
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

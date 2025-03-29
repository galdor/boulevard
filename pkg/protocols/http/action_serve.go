package http

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path"

	"go.n16f.net/bcl"
	"go.n16f.net/boulevard/pkg/boulevard"
)

const (
	DefaultServeActionIndexViewMaxFiles = 1000
	ServeActionTimestampLayout          = "2006-01-02 15:04:05Z"
)

type ServeActionCfg struct {
	Path         *boulevard.FormatString
	IndexFiles   []string
	IndexView    ServeActionIndexViewCfg
	FileNotFound *ServeActionFileNotFoundCfg
}

func (cfg *ServeActionCfg) ReadBCLElement(elt *bcl.Element) error {
	if elt.IsBlock() {
		elt.EntryValues("path", &cfg.Path)

		for _, entry := range elt.FindEntries("index_file") {
			var file string
			entry.Values(&file)
			cfg.IndexFiles = append(cfg.IndexFiles, file)
		}

		elt.MaybeElement("index_view", &cfg.IndexView)

		elt.MaybeBlock("file_not_found", &cfg.FileNotFound)
	} else {
		elt.Values(&cfg.Path)
	}

	return nil
}

type ServeActionIndexViewCfg struct {
	Enabled  bool
	MaxFiles int
}

func (cfg *ServeActionIndexViewCfg) ReadBCLElement(elt *bcl.Element) error {
	if elt.IsBlock() {
		cfg.Enabled = true

		cfg.MaxFiles = DefaultServeActionIndexViewMaxFiles
		elt.MaybeEntryValues("max_files", &cfg.MaxFiles)
	} else {
		elt.Values(&cfg.Enabled)
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

	view *View
}

func NewServeAction(h *Handler, cfg *ServeActionCfg) (*ServeAction, error) {
	view, err := NewView("templates/serve")
	if err != nil {
		return nil, fmt.Errorf("cannot create view: %w", err)
	}

	a := ServeAction{
		Handler: h,
		Cfg:     cfg,

		view: view,
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

	subpath := ctx.Subpath
	if subpath == "" {
		// If the handler did not match a path, there is no subpath in the
		// context, meaning that we are serving what the request URL contains.
		subpath = ctx.Request.URL.Path
	}
	filePath := path.Join(basePath, subpath)

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
			indexFilePath := path.Join(filePath, indexFile)
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

		if a.Cfg.IndexView.Enabled {
			a.serveIndexView(filePath, ctx)
			return
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

func (a *ServeAction) serveIndexView(dirPath string, ctx *RequestContext) {
	entries, err := a.readIndexEntries(dirPath)
	if err != nil {
		ctx.Log.Error("cannot read index entries: %v", err)
		ctx.ReplyError(500)
		return
	}

	// Let us not leak the full server-side path
	relDirPath := ctx.Subpath
	if relDirPath == "" {
		relDirPath = "."
	}

	tplData := struct {
		DirectoryPath string            `json:"directory_path"`
		Entries       []ServeIndexEntry `json:"entries"`

		MaxDisplayedFilenameLength int `json:"-"`
		MTimeLength                int `json:"-"`
		MaxDisplayedSizeLength     int `json:"-"`
	}{
		DirectoryPath: relDirPath,
		Entries:       entries,

		MTimeLength: len(ServeActionTimestampLayout),
	}

	for _, e := range entries {
		tplData.MaxDisplayedFilenameLength =
			max(tplData.MaxDisplayedFilenameLength, len(e.Filename))
		tplData.MaxDisplayedSizeLength = max(tplData.MaxDisplayedSizeLength,
			len(e.DisplayedSize))
	}

	content, err := a.view.Render("index", tplData, ctx)
	if err != nil {
		ctx.Log.Error("cannot render index data: %v", err)
		ctx.ReplyError(500)
		return
	}

	ctx.Reply(200, bytes.NewReader(content))
}

type ServeIndexEntry struct {
	Filename          string `json:"filename"`
	DisplayedFilename string `json:"-"`
	Directory         bool   `json:"directory,omitempty"`
	Size              int64  `json:"size,omitempty"`
	DisplayedSize     string `json:"displayed_size,omitempty"`
	MTime             string `json:"mtime,omitempty"`
}

func (a *ServeAction) readIndexEntries(dirPath string) ([]ServeIndexEntry, error) {
	dirEntries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("cannot read directory %q: %v", dirPath, err)
	}

	idxEntries := make([]ServeIndexEntry, len(dirEntries))
	for i, de := range dirEntries {
		ie := ServeIndexEntry{
			Filename: de.Name(),
		}

		ie.DisplayedFilename = ie.Filename
		if de.IsDir() {
			ie.DisplayedFilename += "/"
			ie.Directory = true
		}

		// Do not fail just because we cannot get file information, we will
		// simply show them as unavailable in the templates.
		if info, err := de.Info(); err == nil {
			if !de.IsDir() {
				ie.Size = info.Size()
				ie.DisplayedSize = a.formatFileSize(ie.Size)
			}

			mtime := info.ModTime()
			ie.MTime = mtime.UTC().Format(ServeActionTimestampLayout)
		}

		idxEntries[i] = ie
	}

	return idxEntries, nil
}

func (a *ServeAction) formatFileSize(size int64) string {
	switch {
	case size < 1_000:
		return fmt.Sprintf("%d     B", size)
	case size < 1_000_000:
		return fmt.Sprintf("%.2fKiB", float64(size)/1e3)
	case size < 1_000_000_000:
		return fmt.Sprintf("%.2fMiB", float64(size)/1e6)
	case size < 1_000_000_000_000:
		return fmt.Sprintf("%.2fGiB", float64(size)/1e9)
	default:
		return fmt.Sprintf("%.2fTiB", float64(size)/1e12)
	}
}

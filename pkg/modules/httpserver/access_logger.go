package httpserver

import (
	"fmt"
	"io"
	"os"

	"go.n16f.net/boulevard/pkg/boulevard"
	"go.n16f.net/ejson"
)

type AccessLoggerCfg struct {
	Path   boulevard.String `json:"path"`
	Format boulevard.String `json:"format"`
}

func (cfg *AccessLoggerCfg) ValidateJSON(v *ejson.Validator) {
	// TODO
}

type AccessLogger struct {
	Cfg *AccessLoggerCfg

	w io.WriteCloser
}

func (mod *Module) NewAccessLogger(cfg *AccessLoggerCfg) (*AccessLogger, error) {
	l := AccessLogger{
		Cfg: cfg,
	}

	filePath := cfg.Path.Expand(mod.Vars)

	flags := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	file, err := os.OpenFile(filePath, flags, 0644)
	if err != nil {
		return nil, fmt.Errorf("cannot open %q: %w", filePath, err)
	}
	l.w = file

	return &l, nil
}

func (l *AccessLogger) Close() error {
	return l.w.Close()
}

func (l *AccessLogger) Log(ctx *RequestContext) error {
	msg := l.Cfg.Format.Expand(ctx.Vars)

	data := append([]byte(msg), '\n')

	_, err := l.w.Write(data)
	return err
}

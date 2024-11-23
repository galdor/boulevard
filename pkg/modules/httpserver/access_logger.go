package httpserver

import (
	"bytes"
	"cmp"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	"go.n16f.net/boulevard/pkg/boulevard"
	"go.n16f.net/ejson"
)

const (
	AccessLoggerFormatCommon = "common"
)

type AccessLoggerCfg struct {
	Path   *boulevard.FormatString `json:"path"`
	Format *boulevard.FormatString `json:"format"`
}

func (cfg *AccessLoggerCfg) ValidateJSON(v *ejson.Validator) {
	v.CheckObject("path", cfg.Path)
	v.CheckObject("format", cfg.Format)
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
	var data []byte

	switch l.Cfg.Format.Value {
	case AccessLoggerFormatCommon:
		data = l.formatMsgCommon(ctx)
	default:
		data = []byte(l.Cfg.Format.Expand(ctx.Vars))
	}

	data = append(data, '\n')

	_, err := l.w.Write(data)
	return err
}

func (l *AccessLogger) formatMsgCommon(ctx *RequestContext) []byte {
	var buf bytes.Buffer

	buf.WriteString(ctx.ClientAddress.String())

	buf.WriteByte(' ')
	buf.WriteString("-")

	buf.WriteByte(' ')
	buf.WriteString(cmp.Or(ctx.Username, "-"))

	buf.WriteByte(' ')
	buf.WriteByte('[')
	buf.WriteString(time.Now().Format("02/Jan/2006:15:04:05 -0700"))
	buf.WriteByte(']')

	buf.WriteByte(' ')
	buf.WriteByte('"')
	buf.WriteString(ctx.Request.Method)
	buf.WriteByte(' ')
	buf.WriteString(ctx.Request.URL.Path)
	buf.WriteByte(' ')
	buf.WriteString(ctx.Request.Proto)
	buf.WriteByte('"')

	buf.WriteByte(' ')
	if ctx.ResponseStatus == 0 {
		buf.WriteByte('-')
	} else {
		buf.WriteString(strconv.Itoa(ctx.ResponseStatus))
	}

	buf.WriteByte(' ')
	buf.WriteString(strconv.Itoa(ctx.ResponseBodySize))

	return buf.Bytes()
}

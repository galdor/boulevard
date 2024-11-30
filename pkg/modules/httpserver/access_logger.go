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
	AccessLoggerFormatCommon   = "common"
	AccessLoggerFormatCombined = "combined"
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

func NewAccessLogger(cfg *AccessLoggerCfg, vars map[string]string) (*AccessLogger, error) {
	l := AccessLogger{
		Cfg: cfg,
	}

	filePath := cfg.Path.Expand(vars)

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
	var buf bytes.Buffer

	switch l.Cfg.Format.Value {
	case AccessLoggerFormatCommon:
		l.formatMsgCommon(ctx, &buf)
	case AccessLoggerFormatCombined:
		l.formatMsgCombined(ctx, &buf)
	default:
		buf.WriteString(l.Cfg.Format.Expand(ctx.Vars))
	}

	buf.WriteByte('\n')

	_, err := l.w.Write(buf.Bytes())
	return err
}

func (l *AccessLogger) formatMsgCommon(ctx *RequestContext, buf *bytes.Buffer) {
	if ctx.ClientAddress == nil {
		buf.WriteByte('-')
	} else {
		buf.WriteString(ctx.ClientAddress.String())
	}

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
	if ctx.ResponseWriter.Status == 0 {
		buf.WriteByte('-')
	} else {
		buf.WriteString(strconv.Itoa(ctx.ResponseWriter.Status))
	}

	buf.WriteByte(' ')
	buf.WriteString(strconv.Itoa(ctx.ResponseWriter.BodySize))
}

func (l *AccessLogger) formatMsgCombined(ctx *RequestContext, buf *bytes.Buffer) {
	l.formatMsgCommon(ctx, buf)

	header := ctx.Request.Header

	buf.WriteByte(' ')
	if referer := header.Get("Referer"); referer == "" {
		buf.WriteByte('-')
	} else {
		fmt.Fprintf(buf, "%q", referer)
	}

	buf.WriteByte(' ')
	if userAgent := header.Get("User-Agent"); userAgent == "" {
		buf.WriteByte('-')
	} else {
		fmt.Fprintf(buf, "%q", userAgent)
	}
}

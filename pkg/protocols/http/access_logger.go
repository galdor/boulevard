package http

import (
	"bytes"
	"cmp"
	"fmt"
	"io"
	"os"
	"strconv"
	"sync"
	"time"

	"go.n16f.net/bcl"
	"go.n16f.net/boulevard/pkg/boulevard"
)

const (
	AccessLoggerFormatCommon   = "common"
	AccessLoggerFormatCombined = "combined"
)

type AccessLoggerCfg struct {
	Path   *boulevard.FormatString
	Format *boulevard.FormatString
}

func (cfg *AccessLoggerCfg) ReadBCLElement(block *bcl.Element) error {
	block.EntryValues("path", &cfg.Path)
	block.EntryValues("format", &cfg.Format)

	return nil
}

type AccessLogger struct {
	Cfg *AccessLoggerCfg

	filePath string
	file     io.WriteCloser
	fileLock sync.RWMutex
}

func NewAccessLogger(cfg *AccessLoggerCfg, vars map[string]string) (*AccessLogger, error) {
	l := AccessLogger{
		Cfg: cfg,

		filePath: cfg.Path.Expand(vars),
	}

	if err := l.Reopen(); err != nil {
		return nil, err
	}

	return &l, nil
}

func (l *AccessLogger) FilePath() string {
	return l.filePath
}

func (l *AccessLogger) Close() error {
	return l.file.Close()
}

func (l *AccessLogger) Reopen() error {
	flags := os.O_WRONLY | os.O_CREATE | os.O_APPEND
	file, err := os.OpenFile(l.filePath, flags, 0644)
	if err != nil {
		return fmt.Errorf("cannot open %q: %w", l.filePath, err)
	}

	l.fileLock.Lock()

	if l.file != nil {
		l.file.Close()
	}
	l.file = file

	l.fileLock.Unlock()

	return nil
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

	l.fileLock.RLock()
	_, err := l.file.Write(buf.Bytes())
	l.fileLock.RUnlock()

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

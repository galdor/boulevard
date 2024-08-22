package httpserver

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"go.n16f.net/log"
)

type RequestContext struct {
	Log            *log.Logger
	Request        *http.Request
	ResponseWriter http.ResponseWriter

	Subpath string // always relative
}

func (ctx *RequestContext) Reply(status int, data io.Reader) {
	ctx.ResponseWriter.WriteHeader(status)

	if data != nil {
		if _, err := io.Copy(ctx.ResponseWriter, data); err != nil {
			ctx.Log.Error("cannot write response body: %v", err)
		}
	}
}

func (ctx *RequestContext) ReplyError(status int) {
	header := ctx.ResponseWriter.Header()
	header.Set("Content-Type", "text/plain")

	msg := fmt.Sprintf("%d %s\n", status, http.StatusText(status))
	ctx.Reply(status, strings.NewReader(msg))
}

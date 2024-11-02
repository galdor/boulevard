package httpserver

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"

	"go.n16f.net/log"
)

type RequestContext struct {
	Log            *log.Logger
	Ctx            context.Context
	Request        *http.Request
	ResponseWriter http.ResponseWriter

	Listener      *Listener
	ClientAddress net.IP
	Host          string
	Subpath       string // always relative

	AccessLogger *AccessLogger
	Auth         Auth

	Vars map[string]string
}

func (ctx *RequestContext) Reply(status int, data io.Reader) {
	ctx.Vars["http.response.status"] = strconv.Itoa(status)

	ctx.ResponseWriter.WriteHeader(status)

	if data != nil {
		if _, err := io.Copy(ctx.ResponseWriter, data); err != nil {
			ctx.Log.Error("cannot write response body: %v", err)
		}
	}
}

func (ctx *RequestContext) ReplyError(status int) {
	header := ctx.ResponseWriter.Header()
	header.Set("Content-Type", MediaTypeText.String())

	msg := fmt.Sprintf("%d %s\n", status, http.StatusText(status))
	ctx.Reply(status, strings.NewReader(msg))
}

func (ctx *RequestContext) NegotiateMediaType(supportedTypes []*MediaType) *MediaType {
	ranges := ctx.AcceptedMediaRanges()
	if len(ranges) == 0 {
		ranges = append(ranges, &MediaRange{}) // accept */* by default
	}

	return NegotiateMediaType(ranges, supportedTypes)
}

func (ctx *RequestContext) AcceptedMediaRanges() []*MediaRange {
	value := ctx.Request.Header.Get("Accept")
	parts := strings.Split(value, ",")

	var ranges []*MediaRange
	for _, part := range parts {
		part = strings.Trim(part, " \t")

		var r MediaRange
		if err := r.Parse(part); err != nil {
			continue
		}

		ranges = append(ranges, &r)
	}

	return ranges
}

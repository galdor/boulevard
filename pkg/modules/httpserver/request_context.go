package httpserver

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"go.n16f.net/boulevard/pkg/httputils"
	"go.n16f.net/log"
)

type RequestContext struct {
	Log            *log.Logger
	Ctx            context.Context
	Request        *http.Request
	ResponseWriter http.ResponseWriter

	Listener          *Listener
	ClientAddress     net.IP
	Host              string
	Subpath           string   // always relative
	ConnectionOptions []string // [1]
	UpgradeProtocols  []string // [1]

	AccessLogger *AccessLogger
	Auth         Auth

	Vars map[string]string

	// [1] Normalized to lower case.
}

func NewRequestContext(req *http.Request, w http.ResponseWriter) *RequestContext {
	ctx := RequestContext{
		Request:        req,
		ResponseWriter: w,

		Vars: make(map[string]string),
	}

	ctx.initConnectionOptions()
	ctx.initUpgradeProtocols()

	return &ctx
}

func (ctx *RequestContext) initConnectionOptions() {
	header := ctx.Request.Header
	connectionField := header.Get("Connection")
	ctx.ConnectionOptions = httputils.SplitTokenList(connectionField, true)
}

func (ctx *RequestContext) initUpgradeProtocols() {
	if ctx.IsHTTP10() {
		// RFC 9110 7.8. "A server that receives an Upgrade header field in an
		// HTTP/1.0 request MUST ignore that Upgrade field".
		return
	}

	if !ctx.IsHTTP1x() {
		// HTTP/2 and HTTP/3 do not support connection upgrades.
		return
	}

	if !slices.Contains(ctx.ConnectionOptions, "upgrade") {
		// RFC 9110 7.8. "A sender of Upgrade MUST also send an "Upgrade"
		// connection option in the Connection header field".
		return
	}

	header := ctx.Request.Header
	upgradeField := header.Get("Upgrade")

	// We do not normalize protocol names on purpose: we do not use them (at
	// least not for the time being) and relay them to the upstream server which
	// may be know that protocol names are case insensitive.
	ctx.UpgradeProtocols = httputils.SplitTokenList(upgradeField, false)
}

func (ctx *RequestContext) IsHTTP1x() bool {
	return ctx.Request.ProtoMajor == 1
}

func (ctx *RequestContext) IsHTTP10() bool {
	return ctx.Request.ProtoMajor == 1 && ctx.Request.ProtoMinor == 0
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

package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net"
	nethttp "net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"go.n16f.net/boulevard/pkg/boulevard"
	"go.n16f.net/boulevard/pkg/httputils"
	"go.n16f.net/boulevard/pkg/netutils"
	"go.n16f.net/log"
	"go.n16f.net/program"
)

type RequestContext struct {
	Log            *log.Logger
	Ctx            context.Context
	Request        *nethttp.Request
	ResponseWriter *httputils.ResponseWriter
	Protocol       *Protocol
	Listener       *boulevard.Listener
	AccessLogger   *AccessLogger
	Auth           Auth

	ClientAddress     net.IP
	Host              string
	Subpath           string   // always relative
	ConnectionOptions []string // [1]
	UpgradeProtocols  []string // [1]
	Username          string   // basic authentication only

	StartTime    time.Time
	ResponseTime time.Duration

	Vars map[string]string

	// [1] Normalized to lower case.
}

func NewRequestContext(cctx context.Context, req *nethttp.Request, w nethttp.ResponseWriter) *RequestContext {
	ctx := RequestContext{
		Ctx:            cctx,
		Request:        req,
		ResponseWriter: httputils.NewResponseWriter(w),

		StartTime: time.Now(),
		Vars:      make(map[string]string),
	}

	ctx.initSubpath()
	ctx.initConnectionOptions()
	ctx.initUpgradeProtocols()
	ctx.initVars()

	return &ctx
}

func (ctx *RequestContext) initSubpath() {
	subpath := ctx.Request.URL.Path
	if len(subpath) > 0 && subpath[0] == '/' {
		subpath = subpath[1:]
	}

	ctx.Subpath = subpath
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

func (ctx *RequestContext) initVars() {
	ctx.Vars["http.request.method"] = strings.ToUpper(ctx.Request.Method)
	ctx.Vars["http.request.uri"] = ctx.Request.URL.String()
	ctx.Vars["http.request.path"] = ctx.Request.URL.Path
	ctx.Vars["http.request.query"] = ctx.Request.URL.RawQuery
}

func (ctx *RequestContext) Recover() {
	if v := recover(); v != nil {
		msg := program.RecoverValueString(v)
		trace := program.StackTrace(1, 20, true)

		ctx.Log.Error("panic: %s\n%s", msg, trace)
		if ctx.ResponseWriter.Status == 0 {
			ctx.ReplyError(500)
		}
	}
}

func (ctx *RequestContext) OnRequestHandled() {
	ctx.ResponseTime = time.Since(ctx.StartTime)

	responseTimeString := strconv.FormatFloat(ctx.ResponseTime.Seconds(),
		'f', -1, 32)
	ctx.Vars["http.response_time"] = responseTimeString

	if ctx.AccessLogger != nil {
		if err := ctx.AccessLogger.Log(ctx); err != nil {
			ctx.Log.Error("cannot log request: %v", err)
		}
	}
}

func (ctx *RequestContext) IdentifyClient() error {
	addr, _, err := netutils.ParseNumericAddress(ctx.Request.RemoteAddr)
	if err != nil {
		return fmt.Errorf("cannot parse remote address %q: %w",
			ctx.Request.RemoteAddr, err)
	}

	ctx.ClientAddress = addr

	ctx.Log.Data["address"] = addr

	ctx.Vars["http.client_address"] = addr.String()

	return nil
}

func (ctx *RequestContext) IdentifyRequestHost() error {
	// Identify the host (hostname or IP address) provided by the client either
	// in the Host header field for HTTP 1.x (defaulting to the host part of the
	// request URI if the Host field is not set in HTTP 1.0) or in the
	// ":authority" pseudo-header field for HTTP 2.
	//
	// Note that it can contain a port (see RFC 9110 7.2. Host and :authority).

	if ctx.Request.Host == "" {
		return fmt.Errorf("missing or empty host")
	}

	host, _, err := net.SplitHostPort(ctx.Request.Host)
	if err != nil {
		host = ctx.Request.Host
	}

	ctx.Host = host

	ctx.Vars["http.request.host"] = ctx.Host

	return nil
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

func (ctx *RequestContext) ReplyJSON(status int, value any) {
	var data []byte

	if value != nil {
		var err error
		data, err = json.MarshalIndent(value, "", "  ")
		if err != nil {
			ctx.Log.Error("cannot encode response body: %v", err)
			ctx.ReplyError(500)
			return
		}
	}

	header := ctx.ResponseWriter.Header()
	header.Set("Content-Type", MediaTypeJSON.String())

	ctx.ResponseWriter.WriteHeader(status)

	if data != nil {
		data = append(data, '\n')
		dataReader := bytes.NewReader(data)

		if _, err := io.Copy(ctx.ResponseWriter, dataReader); err != nil {
			ctx.Log.Error("cannot write response body: %v", err)
		}
	}
}

func (ctx *RequestContext) ReplyError(status int) {
	header := ctx.ResponseWriter.Header()
	header.Set("Content-Type", MediaTypeText.String())

	msg := fmt.Sprintf("%d %s\n", status, nethttp.StatusText(status))
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

func (ctx *RequestContext) LogVariables() {
	keys := slices.Collect(maps.Keys(ctx.Vars))
	slices.Sort(keys)

	width := 0
	for _, key := range keys {
		width = max(width, len(key))
	}

	lines := make([]string, len(keys))
	for i, key := range keys {
		lines[i] = fmt.Sprintf("  %-*s  %s", width, key, ctx.Vars[key])
	}

	ctx.Log.Debug(1, "variables:\n"+strings.Join(lines, "\n"))
}

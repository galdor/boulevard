package httpserver

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"

	"go.n16f.net/boulevard/pkg/httputils"
	"go.n16f.net/ejson"
)

type ReverseProxyActionCfg struct {
	URI string `json:"uri"`
}

func (cfg *ReverseProxyActionCfg) ValidateJSON(v *ejson.Validator) {
	httputils.CheckHTTPURI(v, "uri", cfg.URI)
}

type ReverseProxyAction struct {
	Handler *Handler
	Cfg     ReverseProxyActionCfg

	transport *http.Transport
	uri       *url.URL
}

func NewReverseProxyAction(h *Handler, cfg ReverseProxyActionCfg) (*ReverseProxyAction, error) {
	uri, err := url.Parse(cfg.URI)
	if err != nil {
		return nil, fmt.Errorf("cannot parse URI: %w", err)
	}
	if uri.Scheme == "" {
		uri.Scheme = "http"
	}
	if uri.Host == "" {
		uri.Host = "localhost"
	}
	uri.Path = ""
	uri.Fragment = ""

	dialer := net.Dialer{
		Timeout: 30 * time.Second,
	}

	transport := http.Transport{
		DialContext: dialer.DialContext,

		MaxIdleConns:        250,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     10 * time.Second,

		ExpectContinueTimeout: time.Second,
	}

	a := ReverseProxyAction{
		Handler: h,
		Cfg:     cfg,

		transport: &transport,
		uri:       uri,
	}

	return &a, nil
}

func (a *ReverseProxyAction) Start() error {
	return nil
}

func (a *ReverseProxyAction) Stop() {
	a.transport.CloseIdleConnections()
}

func (a *ReverseProxyAction) HandleRequest(ctx *RequestContext) {
	req := a.rewriteRequest(ctx)

	res, err := a.transport.RoundTrip(req)
	if err != nil {
		ctx.Log.Error("cannot relay request: %v", err)
		ctx.ReplyError(500)
		return
	}
	defer res.Body.Close()

	a.initResponseHeader(ctx, res)
	ctx.ResponseWriter.WriteHeader(res.StatusCode)

	if _, err := io.Copy(ctx.ResponseWriter, res.Body); err != nil {
		ctx.Log.Error("cannot copy response body: %v", err)
		return
	}
}

func (a *ReverseProxyAction) rewriteRequest(ctx *RequestContext) *http.Request {
	req := ctx.Request.Clone(context.Background())
	header := req.Header

	// Rewrite the URI to target the upstream server
	req.URL.Scheme = a.uri.Scheme
	req.URL.Host = a.uri.Host

	a.initRequestHeader(ctx, header)

	return req
}

func (a *ReverseProxyAction) initRequestHeader(ctx *RequestContext, header http.Header) {
	a.deleteRequestHeaderHopByHopFields(ctx, header)
	a.deleteRequestUserAgentField(ctx, header)
	a.setRequestHeaderForwardedFields(ctx, header)
}

func (a *ReverseProxyAction) deleteRequestHeaderHopByHopFields(ctx *RequestContext, header http.Header) {
	// RFC 9110 7.6.1. Connection: "Intermediaries MUST parse a received
	// Connection header field before a message is forwarded and, for each
	// connection-option in this field, remove any header or trailer field(s)
	// from the message with the same name as the connection-option, and then
	// remove the Connection header field itself (or replace it with the
	// intermediary's own control options for the forwarded message)."
	connectionFields := httputils.SplitTokenList(header.Get("Connection"))
	for _, name := range connectionFields {
		header.Del(name)
	}

	// Header fields listed in RFC 2616 (13.5.1 End-to-end and Hop-by-hop
	// Headers) should probably be deleted too.
	var rfc2616Fields = []string{
		"Connection",
		"Keep-Alive",
		"Proxy-Authenticate",
		"Proxy-Authorization",
		"TE",
		"Trailers",
		"Transfer-Encoding",
		"Upgrade",
	}

	for _, name := range rfc2616Fields {
		header.Del(name)
	}
}

func (a *ReverseProxyAction) deleteRequestUserAgentField(ctx *RequestContext, header http.Header) {
	// Disable the user agent automatically set by the net/http module if we do
	// not provide one.
	if userAgent := header.Get("User-Agent"); userAgent == "" {
		header.Set("User-Agent", "")
	}
}

func (a *ReverseProxyAction) setRequestHeaderForwardedFields(ctx *RequestContext, header http.Header) {
	// In theory, everyone should use the Forwarded field (RFC 7239). In
	// practice, I have not every seen anyone relying on it. Ever.

	addrList := header.Get("X-Forwarded-For")
	addrList = httputils.AppendToTokenList(addrList, ctx.ClientAddress.String())
	header.Set("X-Forwarded-For", addrList)

	header.Set("X-Forwarded-Host", ctx.Host)

	var forwardedProto string
	if ctx.Request.TLS == nil {
		forwardedProto = "http"
	} else {
		forwardedProto = "https"
	}
	header.Set("X-Forwarded-Proto", forwardedProto)
}

func (a *ReverseProxyAction) initResponseHeader(ctx *RequestContext, res *http.Response) {
	header := ctx.ResponseWriter.Header()

	for name, fields := range res.Header {
		for _, field := range fields {
			header.Add(name, field)
		}
	}
}

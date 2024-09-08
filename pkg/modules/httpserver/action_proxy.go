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

type ProxyActionCfg struct {
	URI string `json:"uri"`
}

func (cfg *ProxyActionCfg) ValidateJSON(v *ejson.Validator) {
	httputils.CheckHTTPURI(v, "uri", cfg.URI)
}

type ProxyAction struct {
	Handler *Handler
	Cfg     ProxyActionCfg

	transport *http.Transport
	uri       *url.URL
}

func NewProxyAction(h *Handler, cfg ProxyActionCfg) (*ProxyAction, error) {
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

	a := ProxyAction{
		Handler: h,
		Cfg:     cfg,

		transport: &transport,
		uri:       uri,
	}

	return &a, nil
}

func (a *ProxyAction) Start() error {
	return nil
}

func (a *ProxyAction) Stop() {
	a.transport.CloseIdleConnections()
}

func (a *ProxyAction) HandleRequest(ctx *RequestContext) {
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

func (a *ProxyAction) rewriteRequest(ctx *RequestContext) *http.Request {
	req := ctx.Request.Clone(context.Background())
	header := req.Header

	// Rewrite the URI to target the upstream server
	req.URL.Scheme = a.uri.Scheme
	req.URL.Host = a.uri.Host

	a.initRequestHeader(header)

	return req
}

func (a *ProxyAction) initRequestHeader(header http.Header) {
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

	// Disable the user agent automatically set by the net/http module if we do
	// not provide one.
	if userAgent := header.Get("User-Agent"); userAgent == "" {
		header.Set("User-Agent", "")
	}
}

func (a *ProxyAction) initResponseHeader(ctx *RequestContext, res *http.Response) {
	header := ctx.ResponseWriter.Header()

	for name, fields := range res.Header {
		for _, field := range fields {
			header.Add(name, field)
		}
	}
}

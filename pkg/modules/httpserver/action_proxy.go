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
	uri := req.URL
	header := req.Header

	// Rewrite the URI to target the upstream server
	uri.Scheme = a.uri.Scheme
	uri.Host = a.uri.Host

	// Disable the user agent automatically set by the net/http module if we do
	// not provide one.
	if userAgent := header.Get("User-Agent"); userAgent == "" {
		header.Set("User-Agent", "")
	}

	return req
}

func (a *ProxyAction) initResponseHeader(ctx *RequestContext, res *http.Response) {
	header := ctx.ResponseWriter.Header()

	for name, fields := range res.Header {
		for _, field := range fields {
			header.Add(name, field)
		}
	}
}

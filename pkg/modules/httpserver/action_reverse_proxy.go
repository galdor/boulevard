package httpserver

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"go.n16f.net/bcl"
	"go.n16f.net/boulevard/pkg/boulevard"
	"go.n16f.net/boulevard/pkg/httputils"
)

type ReverseProxyActionCfg struct {
	URI            string
	RequestHeader  Header
	ResponseHeader Header
}

func (cfg *ReverseProxyActionCfg) Init(elt *bcl.Element) {
	if elt.IsBlock() {
		elt.EntryValue("uri",
			bcl.WithValueValidation(&cfg.URI, httputils.ValidateBCLHTTPURI))

		cfg.RequestHeader = make(Header)
		for _, entry := range elt.Entries("request_header") {
			var name string
			var value boulevard.FormatString

			if entry.Values(&name, &value) {
				cfg.RequestHeader[name] = &value
			}
		}

		cfg.ResponseHeader = make(Header)
		for _, entry := range elt.Entries("response_header") {
			var name string
			var value boulevard.FormatString

			if entry.Values(&name, &value) {
				cfg.ResponseHeader[name] = &value
			}
		}
	} else {
		elt.Value(
			bcl.WithValueValidation(&cfg.URI, httputils.ValidateBCLHTTPURI))
	}
}

type ReverseProxyAction struct {
	Handler *Handler
	Cfg     *ReverseProxyActionCfg

	uri    *url.URL
	client *httputils.Client
}

func NewReverseProxyAction(h *Handler, cfg *ReverseProxyActionCfg) (*ReverseProxyAction, error) {
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

	tlsCfg := tls.Config{}

	clientCfg := httputils.ClientCfg{
		Scheme: uri.Scheme,
		Host:   uri.Host,

		TLS: &tlsCfg,
	}

	client, err := httputils.NewClient(clientCfg)
	if err != nil {
		return nil, fmt.Errorf("cannot create client: %w", err)
	}

	a := ReverseProxyAction{
		Handler: h,
		Cfg:     cfg,

		uri:    uri,
		client: client,
	}

	return &a, nil
}

func (a *ReverseProxyAction) Start() error {
	return nil
}

func (a *ReverseProxyAction) Stop() {
	a.client.Stop()
}

func (a *ReverseProxyAction) HandleRequest(ctx *RequestContext) {
	req := a.rewriteRequest(ctx)
	a.maybeSetConnectionUpgrade(ctx, req)

	var hijack bool

	conn, err := a.client.AcquireConn()
	if err != nil {
		ctx.Log.Error("cannot acquire connection: %v", err)
		ctx.ReplyError(500)
		return
	}
	defer func() {
		if hijack {
			a.client.HijackConn(conn)
		} else {
			a.client.ReleaseConn(conn)
		}
	}()

	res, err := conn.SendRequest(req)
	if err != nil {
		ctx.Log.Error("cannot send request: %v", err)
		ctx.ReplyError(500)
		conn.Close()
		return
	}
	defer res.Body.Close()

	a.initResponseHeader(ctx, res)
	ctx.Reply(res.StatusCode, nil)

	if a.isConnectionUpgraded(ctx, res) {
		// If we did not ask for a connection upgrade but we are
		// receiving a 101 response acknowledging an upgrade, something
		// is wrong.
		if len(ctx.UpgradeProtocols) == 0 {
			ctx.Log.Error("unexpected upgrade response")
			return
		}

		// Hijack the connection between the client and us
		if err := a.hijackConnection(ctx, conn); err != nil {
			ctx.Log.Error("cannot hijack connection: %v", err)
			return
		}

		hijack = true
	} else {
		if _, err := io.Copy(ctx.ResponseWriter, res.Body); err != nil {
			ctx.Log.Error("cannot copy response body: %v", err)
			return
		}
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

	a.Cfg.RequestHeader.Apply(header, ctx.Vars)
}

func (a *ReverseProxyAction) deleteRequestHeaderHopByHopFields(ctx *RequestContext, header http.Header) {
	// RFC 9110 7.6.1. Connection: "Intermediaries MUST parse a received
	// Connection header field before a message is forwarded and, for each
	// connection-option in this field, remove any header or trailer field(s)
	// from the message with the same name as the connection-option, and then
	// remove the Connection header field itself (or replace it with the
	// intermediary's own control options for the forwarded message)."
	for _, name := range ctx.ConnectionOptions {
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

func (a *ReverseProxyAction) maybeSetConnectionUpgrade(ctx *RequestContext, req *http.Request) {
	// Relay connection upgrade fields to the upstream server
	if len(ctx.UpgradeProtocols) == 0 {
		return
	}

	header := req.Header
	header.Set("Connection", "upgrade")
	header.Set("Upgrade", strings.Join(ctx.UpgradeProtocols, ", "))
}

func (a *ReverseProxyAction) isConnectionUpgraded(ctx *RequestContext, res *http.Response) bool {
	if res.StatusCode != 101 {
		return false
	}

	header := ctx.Request.Header

	connectionField := header.Get("Connection")
	connectionOptions := httputils.SplitTokenList(connectionField, true)
	if !slices.Contains(connectionOptions, "upgrade") {
		return false
	}

	return true
}

func (a *ReverseProxyAction) initResponseHeader(ctx *RequestContext, res *http.Response) {
	header := ctx.ResponseWriter.Header()

	for name, fields := range res.Header {
		for _, field := range fields {
			header.Add(name, field)
		}
	}

	a.Cfg.ResponseHeader.Apply(header, ctx.Vars)
}

func (a *ReverseProxyAction) hijackConnection(ctx *RequestContext, upstreamConn *httputils.ClientConn) error {
	listener := ctx.Listener

	conn, remainingClientData, err := ctx.ResponseWriter.Hijack()
	if err != nil {
		return err
	}

	if n := remainingClientData.Writer.Buffered(); n > 0 {
		_, err := io.CopyN(upstreamConn.Conn, remainingClientData, int64(n))
		if err != nil {
			conn.Close()
			listener.nbConnections.Add(-1)
			return fmt.Errorf("cannot copy data to upstream connection: %w",
				err)
		}
	}

	tcpConn := TCPConnection{
		Module:   listener.Module,
		Listener: listener,
		Log:      ctx.Log.Child("", nil),

		conn:         conn,
		upstreamConn: upstreamConn.Conn,
	}

	listener.registerTCPConnection(&tcpConn)

	listener.wg.Add(2)
	go tcpConn.read()
	go tcpConn.write()

	return nil
}

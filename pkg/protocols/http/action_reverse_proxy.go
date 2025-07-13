package http

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"go.n16f.net/bcl"
	"go.n16f.net/boulevard/pkg/boulevard"
	"go.n16f.net/boulevard/pkg/httputils"
	"go.n16f.net/boulevard/pkg/netutils"
)

type ReverseProxyActionCfg struct {
	// One or the other
	URI              string
	LoadBalancerName string

	RequestHeader  HeaderOps
	ResponseHeader HeaderOps
}

func (cfg *ReverseProxyActionCfg) ReadBCLElement(elt *bcl.Element) error {
	if elt.IsBlock() {
		elt.CheckElementsOneOf("uri", "load_balancer")
		elt.MaybeEntryValues("uri",
			bcl.WithValueValidation(&cfg.URI, httputils.ValidateBCLHTTPURI))
		elt.MaybeEntryValues("load_balancer", &cfg.LoadBalancerName)

		elt.MaybeBlock("request_header", &cfg.RequestHeader)
		elt.MaybeBlock("response_header", &cfg.ResponseHeader)
	} else {
		elt.Values(
			bcl.WithValueValidation(&cfg.URI, httputils.ValidateBCLHTTPURI))
	}

	return nil
}

type ReverseProxyAction struct {
	Handler *Handler
	Cfg     *ReverseProxyActionCfg

	// Single upstream server
	uri    *url.URL
	client *httputils.Client

	// Load balancer
	loadBalancer *boulevard.LoadBalancer
	clients      map[string]*httputils.Client // address -> client
}

func NewReverseProxyAction(h *Handler, cfg *ReverseProxyActionCfg) (*ReverseProxyAction, error) {
	tlsCfg := tls.Config{}

	a := ReverseProxyAction{
		Handler: h,
		Cfg:     cfg,
	}

	if cfg.URI != "" {
		// URI
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

		port := uri.Port()
		if port == "" {
			if strings.ToLower(uri.Scheme) == "http" {
				port = "80"
			} else {
				port = "443"
			}
		}
		address := net.JoinHostPort(uri.Hostname(), port)

		clientCfg := httputils.ClientCfg{
			Scheme:  uri.Scheme,
			Address: address,

			TLS: &tlsCfg,
		}

		client, err := httputils.NewClient(clientCfg)
		if err != nil {
			return nil, fmt.Errorf("cannot create client: %w", err)
		}

		a.uri = uri
		a.client = client
	} else {
		// Load balancer
		serverCfg := h.Protocol.Server.Cfg

		lb := serverCfg.LoadBalancers[cfg.LoadBalancerName]
		if lb == nil {
			return nil, fmt.Errorf("unknown load balancer %q",
				cfg.LoadBalancerName)
		}

		a.clients = make(map[string]*httputils.Client)
		var startedClients []string

		for _, srv := range lb.Servers {
			address := srv.Address.String()

			clientCfg := httputils.ClientCfg{
				Scheme:  "http",
				Address: address,

				TLS: &tlsCfg,
			}

			client, err := httputils.NewClient(clientCfg)
			if err != nil {
				for _, addr := range startedClients {
					a.clients[addr].Stop()
				}

				return nil, fmt.Errorf("cannot create client: %w", err)
			}

			a.clients[address] = client
			startedClients = append(startedClients, address)
		}

		a.loadBalancer = lb
	}

	return &a, nil
}

func (a *ReverseProxyAction) Start() error {
	return nil
}

func (a *ReverseProxyAction) Stop() {
	if a.client != nil {
		a.client.Stop()
	} else {
		for _, client := range a.clients {
			client.Stop()
		}
	}
}

func (a *ReverseProxyAction) HandleRequest(ctx *RequestContext) {
	var client *httputils.Client
	var scheme, address string

	if a.client != nil {
		// Single upstream server
		client = a.client
		scheme = a.uri.Scheme
		address = a.uri.Host
	} else {
		// Load balancer
		address = a.loadBalancer.Address()
		if address == "" {
			ctx.Log.Error("no available upstream server found")
			ctx.ReplyError(503)
			return
		}

		client = a.clients[address]
		scheme = "http"
	}

	req := a.rewriteRequest(ctx, scheme, address)
	a.maybeSetConnectionUpgrade(ctx, req)

	var hijack bool

	conn, err := client.AcquireConn()
	if err != nil {
		ctx.Log.Error("cannot acquire upstream connection: %v", err)

		status := 500
		if errors.Is(err, httputils.ErrNoConnectionAvailable) {
			status = 503
		}

		ctx.ReplyError(status)
		return
	}
	defer func() {
		if hijack {
			client.HijackConn(conn)
		} else {
			client.ReleaseConn(conn)
		}
	}()

	res, err := conn.SendRequest(req)
	if err != nil {
		ctx.Log.Error("cannot send request upstream: %v", err)
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
			if netutils.IsSilentIOError(err) {
				ctx.Log.Debug(1, "cannot copy response body: %v", err)
			} else {
				ctx.Log.Error("cannot copy response body: %v", err)
			}
			return
		}
	}
}

func (a *ReverseProxyAction) rewriteRequest(ctx *RequestContext, scheme, address string) *http.Request {
	req := ctx.Request.Clone(context.Background())
	header := req.Header

	// Rewrite the URI to target the upstream server
	req.URL.Scheme = scheme
	req.URL.Host = address

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
	conn, remainingClientData, err := ctx.ResponseWriter.Hijack()
	if err != nil {
		return err
	}

	if n := remainingClientData.Writer.Buffered(); n > 0 {
		_, err := io.CopyN(upstreamConn.Conn, remainingClientData, int64(n))
		if err != nil {
			conn.Close()
			ctx.Protocol.nbConnections.Add(-1)
			return fmt.Errorf("cannot copy data to upstream connection: %w",
				err)
		}
	}

	tcpConn := TCPConnection{
		Protocol: ctx.Protocol,
		Listener: ctx.Listener,
		Log:      ctx.Log.Child("", nil),

		conn:         conn,
		upstreamConn: upstreamConn.Conn,
	}

	ctx.Protocol.registerTCPConnection(&tcpConn)

	ctx.Protocol.wg.Add(2)
	go tcpConn.read()
	go tcpConn.write()

	return nil
}

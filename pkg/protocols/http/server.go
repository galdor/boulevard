package http

import (
	"io"
	stdlog "log"
	"net"
	"net/http"
	nethttp "net/http"
	"sort"
	"strconv"
	"strings"

	"go.n16f.net/boulevard/pkg/boulevard"
	"go.n16f.net/boulevard/pkg/netutils"
	"go.n16f.net/log"
)

type Server struct {
	Protocol *Protocol
	Listener *boulevard.Listener

	server nethttp.Server
}

func StartServer(p *Protocol, l *boulevard.Listener) (*Server, error) {
	s := Server{
		Protocol: p,
		Listener: l,
	}

	var protocols http.Protocols
	protocols.SetHTTP1(true)
	protocols.SetHTTP2(true)
	if p.Cfg.UnencryptedHTTP2 {
		protocols.SetUnencryptedHTTP2(true)
	}

	s.server = http.Server{
		Addr:      l.Cfg.Address,
		Handler:   &s,
		ErrorLog:  stdlog.New(io.Discard, "", 0),
		ConnState: s.connState,
		Protocols: &protocols,
	}

	if p.Cfg.LogGoServerErrors {
		s.server.ErrorLog = p.Log.StdLogger(log.LevelError)
	}

	p.wg.Add(1)
	go s.serve()

	return &s, nil
}

func (s *Server) Stop() {
	s.server.Shutdown(s.Listener.Ctx)
}

func (s *Server) connState(conn net.Conn, state http.ConnState) {
	switch state {
	case http.StateNew:
		s.Protocol.nbConnections.Add(1)
	case http.StateClosed:
		s.Protocol.nbConnections.Add(-1)
	}
}

func (s *Server) serve() {
	defer s.Protocol.wg.Done()

	err := s.server.Serve(s.Listener.Listener)
	if err != http.ErrServerClosed {
		s.Protocol.Server.Fatal("cannot run HTTP server: %w", err)
		return
	}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	ctx := NewRequestContext(s.Listener.Ctx, req, w)
	ctx.Log = s.Protocol.Log.Child("", nil)
	ctx.Protocol = s.Protocol
	ctx.Listener = s.Listener
	ctx.AccessLogger = s.Protocol.accessLogger

	defer ctx.Recover()
	defer ctx.OnRequestHandled()

	if err := ctx.IdentifyClient(); err != nil {
		// The RemoteAddr field of the Request object not being parsable is an
		// internal issue.
		ctx.Log.Error("cannot identify client: %v", err)
		ctx.ReplyError(500)
		return
	}

	if err := ctx.IdentifyRequestHost(); err != nil {
		// The Host header field being invalid is a client issue
		ctx.ReplyError(400)
		return
	}

	if s.handleTLS(ctx) {
		return
	}

	s.handleHSTS(ctx)

	h := s.Protocol.findHandler(ctx)
	if h == nil {
		ctx.ReplyError(404)
		return
	}

	if rl := ctx.RequestRateLimiter; rl != nil {
		if rl.Update(1, ctx.ClientAddress, ctx.StartTime) == false {
			ctx.ReplyError(429)
			return
		}
	}

	if ctx.Auth != nil {
		if err := ctx.Auth.AuthenticateRequest(ctx); err != nil {
			ctx.Log.Error("cannot authenticate request: %v", err)
			return
		}
	}

	if h.Action == nil {
		ctx.ReplyError(501)
		return
	}

	h.Action.HandleRequest(ctx)

	if s.Protocol.Cfg.DebugLogVariables {
		ctx.LogVariables()
	}
}

func (s *Server) handleTLS(ctx *RequestContext) bool {
	// There is no clear specification about how a server should behave in those
	// situations. We use 426 with a Upgrade header field because it represent
	// what the server expects from the client the best.

	tlsListener := s.Protocol.defaultTLSListener
	tlsHandling := s.Protocol.Cfg.TLSHandling
	header := ctx.ResponseWriter.Header()

	if tlsHandling == TLSHandlingReject && ctx.Request.TLS != nil {
		header.Add("Upgrade", "HTTP/2.0, HTTP/1.1, HTTP/1.0")
		ctx.ReplyError(426)
		return true
	}

	if tlsHandling == TLSHandlingRequire && ctx.Request.TLS == nil {
		if tlsListener != nil {
			versions := tlsListener.Cfg.TLS.SupportedTLSVersions()
			sort.Slice(versions, func(i, j int) bool {
				return versions[i] > versions[j]
			})

			protocols := make([]string, len(versions))
			for i, version := range versions {
				protocols[i] = netutils.HTTPTLSProtocolString(version)
			}

			header.Add("Upgrade", strings.Join(protocols, ", "))
		}

		ctx.ReplyError(426)
		return true
	}

	if tlsHandling == TLSHandlingRedirect && ctx.Request.TLS == nil {
		uri := *ctx.Request.URL
		uri.Scheme = "https"

		uri.Host = ctx.Host
		if tlsListener != nil && tlsListener.Port != 443 {
			uri.Host += ":" + strconv.Itoa(tlsListener.Port)
		}

		header.Add("Location", uri.String())
		ctx.Reply(301, nil)
		return true
	}

	return false
}

func (s *Server) handleHSTS(ctx *RequestContext) {
	if !s.Protocol.Cfg.HSTS {
		return
	}

	// RFC 6797 7.2. "An HSTS Host MUST NOT include the STS header field in HTTP
	// responses conveyed over non-secure transport".
	if ctx.Request.TLS == nil {
		return
	}

	header := ctx.ResponseWriter.Header()
	header.Set("Strict-Transport-Security",
		"max-age=31536000; includeSubDomains")
}

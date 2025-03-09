package http

import (
	"net"
	"net/http"
	nethttp "net/http"

	"go.n16f.net/boulevard/pkg/boulevard"
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

	s.server = http.Server{
		Addr:      l.Cfg.Address,
		Handler:   &s,
		ErrorLog:  p.Log.StdLogger(log.LevelError),
		ConnState: s.connState,
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
		ctx.Log.Error("cannot identify client: %v", err)
		ctx.ReplyError(500)
		return
	}

	if err := ctx.IdentifyRequestHost(); err != nil {
		ctx.Log.Error("cannot identify request host: %v", err)
		ctx.ReplyError(500)
		return
	}

	h := s.Protocol.findHandler(ctx)
	if h == nil {
		ctx.ReplyError(404)
		return
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

	if s.Protocol.Cfg.LogVariables {
		ctx.LogVariables()
	}
}

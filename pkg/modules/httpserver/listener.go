package httpserver

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"sync/atomic"

	"go.n16f.net/boulevard/pkg/netutils"
	"go.n16f.net/log"
	"go.n16f.net/program"
)

type Listener struct {
	Module      *Module
	TCPListener *netutils.TCPListener
	Server      *http.Server

	nbConnections atomic.Int64

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewListener(mod *Module, cfg netutils.TCPListenerCfg) (*Listener, error) {
	cfg.Logger = mod.Log
	cfg.ACMEClient = mod.Data.ACMEClient

	tcpListener, err := netutils.NewTCPListener(cfg)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	l := Listener{
		Module:      mod,
		TCPListener: tcpListener,

		ctx:    ctx,
		cancel: cancel,
	}

	return &l, nil
}

func (l *Listener) Start() error {
	if err := l.TCPListener.Start(); err != nil {
		return err
	}

	l.Server = &http.Server{
		Addr:      l.TCPListener.Cfg.Address,
		Handler:   l,
		ErrorLog:  l.TCPListener.Log.StdLogger(log.LevelError),
		ConnState: l.connState,
	}

	l.wg.Add(1)
	go l.serve()

	return nil
}

func (l *Listener) Stop() {
	l.cancel()
	l.Server.Shutdown(l.ctx)
	l.wg.Wait()

	l.TCPListener.Stop()
}

func (l *Listener) CountConnections() int64 {
	return l.nbConnections.Load()
}

func (l *Listener) fatal(format string, args ...any) {
	err := fmt.Errorf(format, args...)

	select {
	case l.Module.Data.ErrChan <- err:
	case <-l.ctx.Done():
	}
}

func (l *Listener) serve() {
	defer l.wg.Done()

	err := l.Server.Serve(l.TCPListener.Listener)
	if err != http.ErrServerClosed {
		l.fatal("cannot run HTTP server: %w", err)
		return
	}
}

func (l *Listener) connState(conn net.Conn, state http.ConnState) {
	switch state {
	case http.StateNew:
		l.nbConnections.Add(1)
	case http.StateClosed:
		l.nbConnections.Add(-1)
	}
}

func (l *Listener) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	subpath := req.URL.Path
	if len(subpath) > 0 && subpath[0] == '/' {
		subpath = subpath[1:]
	}

	ctx := RequestContext{
		Log: l.Module.Log,

		Request:        req,
		ResponseWriter: w,

		Subpath: subpath,
	}

	defer func() {
		if v := recover(); v != nil {
			msg := program.RecoverValueString(v)
			trace := program.StackTrace(0, 20, true)

			ctx.Log.Error("panic: %s\n%s", msg, trace)
			ctx.ReplyError(500)
		}
	}()

	// Identify the numeric IP address of the client
	clientAddr, _, err := netutils.ParseNumericAddress(req.RemoteAddr)
	if err != nil {
		ctx.Log.Error("cannot parse remote address %q: %v", req.RemoteAddr, err)
		ctx.ReplyError(500)
		return
	}
	ctx.ClientAddress = clientAddr

	// Identify the host (hostname or IP address) provided by the client either
	// in the Host header field for HTTP 1.x (defaulting to the host part of the
	// request URI if the Host field is not set in HTTP 1.0) or in the
	// ":authority" pseudo-header field for HTTP 2. We have to split the address
	// because the net/http module uses the <host>:<port> representation.
	host, _, err := net.SplitHostPort(req.Host)
	if err != nil {
		ctx.Log.Error("cannot parse host %q: %v", req.Host, err)
		ctx.ReplyError(500)
		return
	}
	ctx.Host = host

	// Find the first handler matching the request
	h := l.Module.findHandler(&ctx)
	if h == nil {
		ctx.ReplyError(404)
		return
	}

	// Authenticate the request if necessary
	if ctx.Auth != nil {
		if err := ctx.Auth.AuthenticateRequest(&ctx); err != nil {
			ctx.Log.Error("authentication error: %v", err)
			return
		}
	}

	// Handle the request
	h.Action.HandleRequest(&ctx)
}

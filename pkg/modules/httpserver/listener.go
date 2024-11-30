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
)

type Listener struct {
	Module      *Module
	TCPListener *netutils.TCPListener
	Server      *http.Server

	nbConnections atomic.Int64

	tcpConnections     map[*TCPConnection]struct{}
	tcpConnectionMutex sync.Mutex

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewListener(mod *Module, cfg netutils.TCPListenerCfg) (*Listener, error) {
	cfg.Log = mod.Log
	cfg.ACMEClient = mod.Data.ACMEClient

	tcpListener, err := netutils.NewTCPListener(cfg)
	if err != nil {
		return nil, fmt.Errorf("cannot create TCP listener: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	l := Listener{
		Module:      mod,
		TCPListener: tcpListener,

		tcpConnections: make(map[*TCPConnection]struct{}),

		ctx:    ctx,
		cancel: cancel,
	}

	return &l, nil
}

func (l *Listener) Start() error {
	if err := l.TCPListener.Start(); err != nil {
		return fmt.Errorf("cannot start TCP listener: %w", err)
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

	l.tcpConnectionMutex.Lock()
	for conn := range l.tcpConnections {
		conn.Close()
	}
	l.tcpConnectionMutex.Unlock()

	l.nbConnections.Store(0)
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
	ctx := NewRequestContext(l.ctx, req, w)
	ctx.Log = l.Module.Log.Child("", nil)
	ctx.Listener = l
	ctx.AccessLogger = l.Module.accessLogger

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

	h := l.Module.findHandler(ctx)
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
}

func (l *Listener) registerTCPConnection(c *TCPConnection) {
	l.tcpConnectionMutex.Lock()
	l.tcpConnections[c] = struct{}{}
	l.tcpConnectionMutex.Unlock()
}

func (l *Listener) unregisterTCPConnection(c *TCPConnection) {
	l.tcpConnectionMutex.Lock()
	delete(l.tcpConnections, c)
	l.tcpConnectionMutex.Unlock()
}

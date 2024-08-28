package httpserver

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"sync/atomic"

	"go.n16f.net/boulevard/pkg/netutils"
	"go.n16f.net/boulevard/pkg/utils"
	"go.n16f.net/log"
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
			msg := utils.RecoverValueString(v)
			trace := utils.StackTrace(2, 20, true)

			ctx.Log.Error("panic: %s\n%s", msg, trace)
			ctx.ReplyError(500)
		}
	}()

	h := l.Module.findHandler(&ctx)
	if h == nil {
		ctx.ReplyError(404)
		return
	}

	h.Action.HandleRequest(&ctx)
}

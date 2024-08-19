package httpserver

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"

	"go.n16f.net/boulevard/pkg/netutils"
	"go.n16f.net/ejson"
	"go.n16f.net/log"
)

type ListenerCfg struct {
	Address string           `json:"address"`
	TLS     *netutils.TLSCfg `json:"tls,omitempty"`
}

func (cfg *ListenerCfg) ValidateJSON(v *ejson.Validator) {
	// TODO valid address string
	v.CheckStringNotEmpty("address", cfg.Address)
	v.CheckOptionalObject("tls", cfg.TLS)
}

type Listener struct {
	Module *Module
	Cfg    ListenerCfg
	Log    *log.Logger

	server *http.Server

	wg sync.WaitGroup
}

func NewListener(mod *Module, cfg ListenerCfg) *Listener {
	l := Listener{
		Module: mod,
		Cfg:    cfg,
	}

	return &l
}

func (l *Listener) Start() error {
	l.Log = l.Module.Log

	listener, err := netutils.TCPListen(l.Cfg.Address, l.Cfg.TLS)
	if err != nil {
		return fmt.Errorf("cannot listen on %q: %w", l.Cfg.Address, err)
	}

	l.Log.Info("listening on %q", l.Cfg.Address)

	l.server = &http.Server{
		Addr:     l.Cfg.Address,
		ErrorLog: l.Log.StdLogger(log.LevelError),
	}

	l.wg.Add(1)
	go l.serve(listener)

	return nil
}

func (l *Listener) Stop() {
	l.server.Shutdown(context.Background())
	l.wg.Wait()
}

func (l *Listener) serve(listener net.Listener) {
	defer l.wg.Done()

	if err := l.server.Serve(listener); err != http.ErrServerClosed {
		l.Log.Error("cannot run HTTP server: %v", err)
	}
}

package httpserver

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"

	"go.n16f.net/boulevard/pkg/netutils"
	"go.n16f.net/ejson"
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
	Cfg ListenerCfg

	server *http.Server

	wg sync.WaitGroup
}

func NewListener(cfg ListenerCfg) *Listener {
	l := Listener{
		Cfg: cfg,
	}

	return &l
}

func (l *Listener) Start() error {
	listener, err := netutils.TCPListen(l.Cfg.Address, l.Cfg.TLS)
	if err != nil {
		return fmt.Errorf("cannot listen on %q: %w", l.Cfg.Address, err)
	}

	l.server = &http.Server{
		Addr: l.Cfg.Address,
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
		// TODO
		// l.Log.Error("cannot run HTTP server: %v", err)
	}
}

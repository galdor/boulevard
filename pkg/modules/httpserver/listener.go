package httpserver

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"

	"go.n16f.net/acme"
	"go.n16f.net/boulevard/pkg/netutils"
	"go.n16f.net/ejson"
	"go.n16f.net/log"
)

type ListenerCfg struct {
	Address string           `json:"address"`
	TLS     *netutils.TLSCfg `json:"tls,omitempty"`
}

func (cfg *ListenerCfg) ValidateJSON(v *ejson.Validator) {
	v.CheckListenAddress("address", cfg.Address)
	v.CheckOptionalObject("tls", cfg.TLS)
}

type Listener struct {
	Module *Module
	Cfg    ListenerCfg
	Log    *log.Logger

	server *http.Server

	ctx    context.Context
	cancel context.CancelFunc

	wg sync.WaitGroup
}

func NewListener(mod *Module, cfg ListenerCfg) (*Listener, error) {
	if cfg.TLS != nil {
		if mod.acmeClient == nil {
			return nil, fmt.Errorf("missing ACME client for TLS support")
		}
	}

	ctx, cancel := context.WithCancel(context.Background())

	l := Listener{
		Module: mod,
		Cfg:    cfg,

		ctx:    ctx,
		cancel: cancel,
	}

	return &l, nil
}

func (l *Listener) Start() error {
	l.Log = l.Module.Log

	if cfg := l.Cfg.TLS; cfg != nil {
		client := l.Module.acmeClient
		certName := cfg.CertificateName

		ids := make([]acme.Identifier, len(cfg.Domains))
		for i, domain := range cfg.Domains {
			ids[i] = acme.Identifier{
				Type:  acme.IdentifierTypeDNS,
				Value: domain,
			}
		}

		validity := 30

		eventChan, err := client.RequestCertificate(l.ctx, certName, ids,
			validity)
		if err != nil {
			return fmt.Errorf("cannot request TLS certificate: %v", err)
		}

		go func() {
			for ev := range eventChan {
				if ev.Error != nil {
					l.Log.Error("TLS certificate provisioning error: %v",
						ev.Error)
					l.cancel()
				}
			}
		}()

		certData := client.WaitForCertificate(l.ctx, certName)
		if certData == nil {
			return fmt.Errorf("startup interrupted")
		}

		cfg.GetCertificateFunc = client.GetTLSCertificateFunc(certName)
	}

	listener, err := netutils.TCPListen(l.Cfg.Address, l.Cfg.TLS)
	if err != nil {
		l.cancel()
		return fmt.Errorf("cannot listen on %q: %w", l.Cfg.Address, err)
	}

	l.Log.Info("listening on %q", l.Cfg.Address)

	l.server = &http.Server{
		Addr:     l.Cfg.Address,
		Handler:  l.Module,
		ErrorLog: l.Log.StdLogger(log.LevelError),
	}

	l.wg.Add(1)
	go l.serve(listener)

	return nil
}

func (l *Listener) Stop() {
	l.cancel()
	l.server.Shutdown(context.Background())
	l.wg.Wait()
}

func (l *Listener) serve(listener net.Listener) {
	defer l.wg.Done()

	if err := l.server.Serve(listener); err != http.ErrServerClosed {
		l.Module.errChan <- fmt.Errorf("cannot run HTTP server: %v", err)
	}
}

package netutils

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"

	"go.n16f.net/acme"
	"go.n16f.net/ejson"
	"go.n16f.net/log"
)

type TCPListenerCfg struct {
	Address string  `json:"address"`
	TLS     *TLSCfg `json:"tls,omitempty"`

	Logger     *log.Logger  `json:"-"` // [1]
	ACMEClient *acme.Client `json:"-"` // [1]

	// [1] Provided by the caller of NewListener.
}

func (cfg *TCPListenerCfg) ValidateJSON(v *ejson.Validator) {
	v.CheckListenAddress("address", cfg.Address)
	v.CheckOptionalObject("tls", cfg.TLS)
}

type TCPListener struct {
	Cfg TCPListenerCfg
	Log *log.Logger

	Listener net.Listener

	ctx    context.Context
	cancel context.CancelFunc
}

func NewTCPListener(ctx context.Context, cfg TCPListenerCfg) (*TCPListener, error) {
	if cfg.TLS != nil {
		if cfg.ACMEClient == nil {
			return nil, fmt.Errorf("missing ACME client for TLS support")
		}
	}

	ctx2, cancel := context.WithCancel(ctx)

	l := TCPListener{
		Cfg: cfg,
		Log: cfg.Logger,

		ctx:    ctx2,
		cancel: cancel,
	}

	return &l, nil
}

func (l *TCPListener) Start() error {
	if cfg := l.Cfg.TLS; cfg != nil {
		client := l.Cfg.ACMEClient
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

	tcpListener, err := net.Listen("tcp", l.Cfg.Address)
	if err != nil {
		l.cancel()
		return fmt.Errorf("cannot create TCP listener: %w", err)
	}

	if l.Cfg.TLS == nil {
		l.Listener = tcpListener
	} else {
		l.Listener = tls.NewListener(tcpListener, l.Cfg.TLS.NetTLSConfig())
	}

	l.Log.Info("listening on %q", l.Cfg.Address)

	return nil
}

func (l *TCPListener) Stop() {
	l.cancel()
	l.Listener.Close()
}

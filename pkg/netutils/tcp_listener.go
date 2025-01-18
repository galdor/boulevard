package netutils

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"strconv"

	"go.n16f.net/acme"
	"go.n16f.net/bcl"
	"go.n16f.net/log"
)

type TCPListenerCfg struct {
	Log        *log.Logger
	ACMEClient *acme.Client

	Address string
	TLS     *TLSCfg
}

func (cfg *TCPListenerCfg) Init(block *bcl.Element) {
	block.EntryValue("address",
		bcl.WithValueValidation(&cfg.Address, ValidateBCLAddress))

	if block := block.MaybeBlock("tls"); block != nil {
		cfg.TLS = new(TLSCfg)
		cfg.TLS.Init(block)
	}
}

type TCPListener struct {
	Cfg TCPListenerCfg
	Log *log.Logger

	Port     int
	Listener net.Listener

	ctx    context.Context
	cancel context.CancelFunc
}

func NewTCPListener(cfg TCPListenerCfg) (*TCPListener, error) {
	if cfg.TLS != nil {
		if cfg.ACMEClient == nil {
			return nil, fmt.Errorf("missing ACME client for TLS support")
		}
	}

	_, port, err := net.SplitHostPort(cfg.Address)
	if err != nil {
		return nil, fmt.Errorf("invalid address: %w", err)
	}
	portNumber, err := strconv.ParseInt(port, 10, 64)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return nil, fmt.Errorf("invalid port %q", port)
	}

	ctx, cancel := context.WithCancel(context.Background())

	l := TCPListener{
		Cfg: cfg,
		Log: cfg.Log,

		Port: int(portNumber),

		ctx:    ctx,
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

func (l *TCPListener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err != nil {
		return nil, UnwrapOpError(err, "accept")
	}

	return conn, nil
}

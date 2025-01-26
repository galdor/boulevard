package netutils

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"os"
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

func (cfg *TCPListenerCfg) ReadBCLElement(block *bcl.Element) error {
	block.EntryValue("address",
		bcl.WithValueValidation(&cfg.Address, ValidateBCLAddress))
	block.MaybeBlock("tls", &cfg.TLS)
	return nil
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
		if len(cfg.TLS.Domains) > 0 {
			if cfg.ACMEClient == nil {
				return nil, fmt.Errorf("missing ACME client for TLS support")
			}
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
	var tlsCfg *tls.Config
	var err error
	if cfg := l.Cfg.TLS; cfg != nil {
		if len(cfg.Domains) > 0 {
			tlsCfg, err = l.acmeTLSCfg()
		} else {
			tlsCfg, err = l.localTLSCfg()
		}

		if len(cfg.CipherSuites) > 0 {
			tlsCfg.CipherSuites = cfg.CipherSuites
		}
	}
	if err != nil {
		return err
	}

	tcpListener, err := net.Listen("tcp", l.Cfg.Address)
	if err != nil {
		l.cancel()
		return fmt.Errorf("cannot create TCP listener: %w", err)
	}

	if tlsCfg == nil {
		l.Listener = tcpListener
	} else {
		l.Listener = tls.NewListener(tcpListener, tlsCfg)
	}

	l.Log.Info("listening on %q", l.Cfg.Address)

	return nil
}

func (l *TCPListener) acmeTLSCfg() (*tls.Config, error) {
	cfg := l.Cfg.TLS

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
		return nil, fmt.Errorf("cannot request TLS certificate: %v", err)
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
		return nil, fmt.Errorf("startup interrupted")
	}

	tlsCfg := tls.Config{
		GetCertificate: client.GetTLSCertificateFunc(certName),
	}

	return &tlsCfg, nil
}

func (l *TCPListener) localTLSCfg() (*tls.Config, error) {
	cfg := l.Cfg.TLS

	certPath := cfg.CertificateFile
	certData, err := os.ReadFile(certPath)
	if err != nil {
		return nil, fmt.Errorf("cannot load certificate from %q: %w",
			certPath, err)
	}

	keyPath := cfg.PrivateKeyFile
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("cannot load private key from %q: %w",
			keyPath, err)
	}

	// This one illustrates the problem with battery-included languages: yes you
	// have a nice magical function taking care of everything, but it comes with
	// error messages that may or may not be useful to your end users. To be
	// improved one day.
	cert, err := tls.X509KeyPair(certData, keyData)
	if err != nil {
		return nil, fmt.Errorf("invalid certificate and/or private key: %w",
			err)
	}

	tlsCfg := tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	return &tlsCfg, nil
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

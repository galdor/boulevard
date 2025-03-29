package boulevard

import (
	"context"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"slices"
	"strconv"
	"strings"

	"go.n16f.net/acme"
	"go.n16f.net/bcl"
	"go.n16f.net/boulevard/pkg/netutils"
	"go.n16f.net/log"
	"golang.org/x/crypto/sha3"
)

type ListenerCfg struct {
	Address string
	TLS     *netutils.TLSCfg

	// Set by the caller of StartListener
	Log        *log.Logger
	ACMEClient *acme.Client
}

func (cfg *ListenerCfg) ReadBCLElement(block *bcl.Element) error {
	block.EntryValues("address",
		bcl.WithValueValidation(&cfg.Address, netutils.ValidateBCLAddress))
	block.MaybeBlock("tls", &cfg.TLS)
	return nil
}

type Listener struct {
	Cfg      *ListenerCfg
	Log      *log.Logger
	Server   *Server
	Port     int
	Listener net.Listener

	Ctx    context.Context
	cancel context.CancelFunc
}

func StartListener(server *Server, cfg *ListenerCfg) (*Listener, error) {
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

	l := Listener{
		Cfg:    cfg,
		Log:    cfg.Log,
		Server: server,
		Port:   int(portNumber),
	}

	l.Ctx, l.cancel = context.WithCancel(context.Background())

	if err := l.listen(); err != nil {
		l.cancel()
		return nil, err
	}

	return &l, nil
}

func (l *Listener) listen() error {
	var tlsCfg *tls.Config
	var err error

	if cfg := l.Cfg.TLS; cfg != nil {
		if len(cfg.Domains) > 0 {
			tlsCfg, err = l.acmeTLSCfg()
		} else {
			tlsCfg, err = l.localTLSCfg()
		}

		if err == nil {
			tlsCfg.MinVersion = cfg.MinVersion
			tlsCfg.MaxVersion = cfg.MaxVersion

			if len(cfg.CipherSuites) > 0 {
				tlsCfg.CipherSuites = cfg.CipherSuites
			}
		}
	}
	if err != nil {
		return err
	}

	tcpListener, err := net.Listen("tcp", l.Cfg.Address)
	if err != nil {
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

func (l *Listener) acmeTLSCfg() (*tls.Config, error) {
	cfg := l.Cfg.TLS

	client := l.Cfg.ACMEClient
	certName := l.acmeCertificateName()

	ids := make([]acme.Identifier, len(cfg.Domains))
	for i, domain := range cfg.Domains {
		ids[i] = acme.Identifier{
			Type:  acme.IdentifierTypeDNS,
			Value: domain,
		}
	}

	// No point to bother with a validity period, Let's Encrypt does not support
	// NotBefore/NotAfter. We will make it a setting if someone wants to use
	// another ACME provider that supports it.
	validity := 0
	eventChan, err := client.RequestCertificate(l.Ctx, certName, ids, validity)
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

	certData := client.WaitForCertificate(l.Ctx, certName)
	if certData == nil {
		return nil, fmt.Errorf("startup interrupted")
	}

	tlsCfg := tls.Config{
		GetCertificate: client.GetTLSCertificateFunc(certName),
	}

	return &tlsCfg, nil
}

func (l *Listener) localTLSCfg() (*tls.Config, error) {
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

func (l *Listener) Stop() {
	l.Listener.Close() // interrupt Accept
	l.cancel()
}

func (l *Listener) Status() *ListenerStatus {
	status := ListenerStatus{
		Address: l.Cfg.Address,
	}

	if l.Cfg.TLS != nil {
		status.TLS = true
		status.ACME = len(l.Cfg.TLS.Domains) > 0
		status.ACMEDomains = slices.Clone(l.Cfg.TLS.Domains)
	}

	return &status
}

func (l *Listener) acmeCertificateName() string {
	// The hash is not about security, it simply is about producing a
	// reasonably-sized unique identifier for the list of domains.

	serverName := l.Server.Cfg.Name
	key := strings.Join(l.Cfg.TLS.Domains, "\x1f")

	var hash [16]byte
	sha3.ShakeSum128(hash[:], []byte(key))

	return fmt.Sprintf("%s-%s", serverName, hex.EncodeToString(hash[:]))
}

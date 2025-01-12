package netutils

import (
	"crypto/tls"

	"go.n16f.net/acme"
	"go.n16f.net/bcl"
)

type TLSCfg struct {
	CertificateName    string
	GetCertificateFunc acme.GetTLSCertificateFunc

	Domains []string
}

func (cfg *TLSCfg) Init(block *bcl.Element) {
	// TODO Validate all domains
	for _, entry := range block.Entries("domain") {
		var domain string
		entry.Value(&domain)
		cfg.Domains = append(cfg.Domains, domain)
	}
}

func (cfg *TLSCfg) NetTLSConfig() *tls.Config {
	netCfg := tls.Config{
		GetCertificate: cfg.GetCertificateFunc,
	}

	return &netCfg
}

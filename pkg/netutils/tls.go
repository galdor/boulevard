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
	if block.CheckEntryMinValues("domains", 1) {
		block.EntryValue("domains", &cfg.Domains)
	}
}

func (cfg *TLSCfg) NetTLSConfig() *tls.Config {
	netCfg := tls.Config{
		GetCertificate: cfg.GetCertificateFunc,
	}

	return &netCfg
}

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

func (cfg *TLSCfg) ReadBCLElement(block *bcl.Element) error {
	for _, entry := range block.FindEntries("domain") {
		var domain string
		entry.Value(bcl.WithValueValidation(&domain, ValidateBCLDomainName))
		cfg.Domains = append(cfg.Domains, domain)
	}

	return nil
}

func (cfg *TLSCfg) NetTLSConfig() *tls.Config {
	netCfg := tls.Config{
		GetCertificate: cfg.GetCertificateFunc,
	}

	return &netCfg
}

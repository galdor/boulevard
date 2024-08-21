package netutils

import (
	"crypto/tls"

	"go.n16f.net/acme"
	"go.n16f.net/ejson"
)

type TLSCfg struct {
	CertificateName    string                     `json:"-"`
	GetCertificateFunc acme.GetTLSCertificateFunc `json:"-"`

	Domains []string `json:"domains"`
}

func (cfg *TLSCfg) ValidateJSON(v *ejson.Validator) {
	v.CheckArrayNotEmpty("domains", cfg.Domains)
	v.WithChild("domains", func() {
		for i, domain := range cfg.Domains {
			v.CheckDomainName(i, domain)
		}
	})
}

func (cfg *TLSCfg) NetTLSConfig() *tls.Config {
	netCfg := tls.Config{
		GetCertificate: cfg.GetCertificateFunc,
	}

	return &netCfg
}

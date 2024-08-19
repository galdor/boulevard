package netutils

import (
	"crypto/tls"

	"go.n16f.net/ejson"
)

type TLSCfg struct {
	Domains []string `json:"domains"`
}

func (cfg *TLSCfg) ValidateJSON(v *ejson.Validator) {
	v.CheckArrayNotEmpty("domains", cfg.Domains)
	v.WithChild("domains", func() {
		for i, domain := range cfg.Domains {
			// TODO valid domain name
			v.CheckStringNotEmpty(i, domain)
		}
	})
}

func (cfg *TLSCfg) NetTLSConfig() *tls.Config {
	netCfg := tls.Config{}

	return &netCfg
}

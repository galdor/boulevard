package netutils

import (
	"fmt"

	"go.n16f.net/bcl"
)

type TLSCfg struct {
	CertificateName string

	// ACME
	Domains []string

	// Manual configuration
	CertificateFile string
	PrivateKeyFile  string
}

func (cfg *TLSCfg) ReadBCLElement(block *bcl.Element) error {
	if acmeBlock := block.FindBlock("acme"); acmeBlock != nil {
		for _, entry := range acmeBlock.FindEntries("domain") {
			var domain string
			entry.Value(bcl.WithValueValidation(&domain, ValidateBCLDomainName))
			cfg.Domains = append(cfg.Domains, domain)
		}

		if len(cfg.Domains) == 0 {
			return fmt.Errorf("ACME configuration does no contain any domain")
		}
	} else {
		block.EntryValue("certificate_file", &cfg.CertificateFile)
		block.EntryValue("private_key_file", &cfg.PrivateKeyFile)
	}

	return nil
}

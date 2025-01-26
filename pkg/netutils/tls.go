package netutils

import (
	"crypto/tls"
	"fmt"

	"go.n16f.net/bcl"
)

var tlsCipherSuites map[string]uint16

func init() {
	tlsCipherSuites = make(map[string]uint16)
	suites := append(tls.CipherSuites(), tls.InsecureCipherSuites()...)
	for _, suite := range suites {
		tlsCipherSuites[suite.Name] = suite.ID
	}
}

type TLSCfg struct {
	CertificateName string

	// ACME
	Domains []string

	// Manual configuration
	CertificateFile string
	PrivateKeyFile  string
	CipherSuites    []uint16
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

		for _, entry := range block.FindEntries("cipher_suite") {
			var name string
			entry.Value(bcl.WithValueValidation(&name,
				ValidateBCLTLSCipherSuite))
			cfg.CipherSuites = append(cfg.CipherSuites, tlsCipherSuites[name])
		}
	}

	return nil
}

func ValidateBCLTLSCipherSuite(v any) error {
	name := v.(string)

	if _, found := tlsCipherSuites[name]; !found {
		return fmt.Errorf("unknown TLS cipher suite")
	}

	return nil
}

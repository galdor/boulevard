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
	MinVersion      uint16
	MaxVersion      uint16
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

		var minVersion string
		block.MaybeEntryValue("min_version",
			bcl.WithValueValidation(&minVersion, ValidateBCLTLSVersion))
		cfg.MinVersion, _ = ParseTLSVersion(minVersion)

		var maxVersion string
		block.MaybeEntryValue("max_version",
			bcl.WithValueValidation(&maxVersion, ValidateBCLTLSVersion))
		cfg.MaxVersion, _ = ParseTLSVersion(maxVersion)

		for _, entry := range block.FindEntries("cipher_suite") {
			var name string
			entry.Value(bcl.WithValueValidation(&name,
				ValidateBCLTLSCipherSuite))
			cfg.CipherSuites = append(cfg.CipherSuites, tlsCipherSuites[name])
		}
	}

	return nil
}

func ValidateBCLTLSVersion(v any) error {
	name := v.(string)
	_, err := ParseTLSVersion(name)
	return err
}

func ParseTLSVersion(s string) (uint16, error) {
	switch s {
	case "1.0":
		return tls.VersionTLS10, nil
	case "1.1":
		return tls.VersionTLS11, nil
	case "1.2":
		return tls.VersionTLS12, nil
	case "1.3":
		return tls.VersionTLS13, nil
	default:
		return 0, fmt.Errorf("invalid TLS version")
	}
}

func ValidateBCLTLSCipherSuite(v any) error {
	name := v.(string)

	if _, found := tlsCipherSuites[name]; !found {
		return fmt.Errorf("unknown TLS cipher suite")
	}

	return nil
}

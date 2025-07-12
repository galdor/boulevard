package netutils

import (
	"crypto/tls"
	"fmt"

	"go.n16f.net/bcl"
	"go.n16f.net/program"
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
			entry.Values(bcl.WithValueValidation(&domain,
				ValidateBCLDomainName))
			cfg.Domains = append(cfg.Domains, domain)
		}

		if len(cfg.Domains) == 0 {
			acmeBlock.AddSimpleValidationError("ACME configuration does no " +
				"contain any domain")
		}
	} else {
		block.EntryValues("certificate_file", &cfg.CertificateFile)
		block.EntryValues("private_key_file", &cfg.PrivateKeyFile)
	}

	var minVersion string
	block.MaybeEntryValues("min_version",
		bcl.WithValueValidation(&minVersion, ValidateBCLTLSVersion))
	cfg.MinVersion, _ = ParseTLSVersion(minVersion)

	var maxVersion string
	block.MaybeEntryValues("max_version",
		bcl.WithValueValidation(&maxVersion, ValidateBCLTLSVersion))
	cfg.MaxVersion, _ = ParseTLSVersion(maxVersion)

	for _, entry := range block.FindEntries("cipher_suite") {
		var name string
		entry.Values(bcl.WithValueValidation(&name,
			ValidateBCLTLSCipherSuite))
		cfg.CipherSuites = append(cfg.CipherSuites, tlsCipherSuites[name])
	}

	return nil
}

func (cfg *TLSCfg) SupportedTLSVersions() []uint16 {
	versions := []uint16{
		tls.VersionTLS10,
		tls.VersionTLS11,
		tls.VersionTLS12,
		tls.VersionTLS13,
	}

	start := 0
	switch cfg.MinVersion {
	case tls.VersionTLS11:
		start = 1
	case tls.VersionTLS12:
		start = 2
	case tls.VersionTLS13:
		start = 3
	}

	end := 4
	switch cfg.MaxVersion {
	case tls.VersionTLS10:
		end = 1
	case tls.VersionTLS11:
		end = 2
	case tls.VersionTLS12:
		end = 3
	}

	if end < start {
		return nil
	}

	return versions[start:end]
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

func HTTPTLSProtocolString(version uint16) (s string) {
	switch version {
	case tls.VersionTLS10:
		s = "TLS/1.0"
	case tls.VersionTLS11:
		s = "TLS/1.1"
	case tls.VersionTLS12:
		s = "TLS/1.2"
	case tls.VersionTLS13:
		s = "TLS/1.3"
	default:
		program.Panic("unhandled TLS version %d", version)
	}

	return
}

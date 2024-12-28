package netutils

import (
	"bytes"
	"fmt"
	"strings"
)

type DomainNamePattern struct {
	Labels []DomainNamePatternLabel
}

type DomainNamePatternLabel struct {
	Value string // empty if "*" wildcard
}

func (p *DomainNamePattern) String() string {
	var buf bytes.Buffer

	for _, l := range p.Labels {
		if l.Value == "" {
			buf.WriteByte('*')
		} else {
			buf.WriteString(l.Value)
		}

		buf.WriteByte('.')
	}

	return buf.String()
}

func (p *DomainNamePattern) Parse(s string) error {
	if s[len(s)-1] == '.' {
		s = s[:len(s)-1]
	}

	if s == "" {
		return fmt.Errorf("empty pattern")
	}

	labelStrings := strings.Split(s, ".")
	p.Labels = make([]DomainNamePatternLabel, len(labelStrings))

	for i, labelString := range labelStrings {
		var label DomainNamePatternLabel

		if labelString != "*" {
			label.Value = labelString
		}

		p.Labels[i] = label
	}

	return nil
}

func (p *DomainNamePattern) Match(domainName string) bool {
	if domainName[len(domainName)-1] == '.' {
		domainName = domainName[:len(domainName)-1]
	}

	if len(domainName) == 0 {
		return false
	}

	labels := strings.Split(domainName, ".")
	if len(labels) != len(p.Labels) {
		return false
	}

	for i, label := range labels {
		patternLabel := p.Labels[i]
		if patternLabel.Value != "" && label != patternLabel.Value {
			return false
		}
	}

	return true
}

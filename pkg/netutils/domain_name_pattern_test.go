package netutils

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDomainNamePatternParse(t *testing.T) {
	assert := assert.New(t)

	tests := []struct {
		s string
		p DomainNamePattern
	}{
		{"com",
			DomainNamePattern{
				Labels: []DomainNamePatternLabel{
					{Value: "com"},
				},
			},
		},
		{"com.",
			DomainNamePattern{
				Labels: []DomainNamePatternLabel{
					{Value: "com"},
				},
			},
		},
		{"example.com",
			DomainNamePattern{
				Labels: []DomainNamePatternLabel{
					{Value: "example"},
					{Value: "com"},
				},
			},
		},
		{"example.com.",
			DomainNamePattern{
				Labels: []DomainNamePatternLabel{
					{Value: "example"},
					{Value: "com"},
				},
			},
		},
		{"foo.example.com",
			DomainNamePattern{
				Labels: []DomainNamePatternLabel{
					{Value: "foo"},
					{Value: "example"},
					{Value: "com"},
				},
			},
		},
		{"foo.example.com.",
			DomainNamePattern{
				Labels: []DomainNamePatternLabel{
					{Value: "foo"},
					{Value: "example"},
					{Value: "com"},
				},
			},
		},
	}

	for _, test := range tests {
		label := test.s

		var p DomainNamePattern
		err := p.Parse(test.s)
		if assert.NoError(err, label) {
			assert.Equal(test.p, p, label)
		}
	}
}

func TestDomainNamePatternMatch(t *testing.T) {
	assert := assert.New(t)

	tests := []struct {
		pattern    string
		domainName string
		match      bool
	}{
		{"com", "com", true},
		{"com", "com.", true},
		{"com.", "com", true},
		{"com.", "com.", true},
		{"example.com", "example.com", true},
		{"example.com", "example.com.", true},
		{"example.com.", "example.com", true},
		{"example.com.", "example.com.", true},
		{"example.com", "com", false},
		{"example.com", "com.", false},
		{"example.com.", "com", false},
		{"example.com.", "com.", false},
		{"*", "com", true},
		{"*", "com.", true},
		{"*.", "com", true},
		{"*.", "com.", true},
		{"*", "example.net", false},
		{"*", ".", false},
		{"*.com", "example.com", true},
		{"*.com", "example.com.", true},
		{"*.com.", "example.com", true},
		{"*.com.", "example.com.", true},
		{"*.com", "example.net", false},
		{"*.com", "foo.example.com", false},
		{"foo.*.com", "foo.example.com", true},
		{"foo.*.com", "foo.example.com.", true},
		{"foo.*.com.", "foo.example.com", true},
		{"foo.*.com.", "foo.example.com.", true},
		{"foo.*.com", "foo.example.net", false},
		{"foo.*.com", "bar.example.com", false},
	}

	for _, test := range tests {
		label := fmt.Sprintf("pattern %q, domain name %q",
			test.pattern, test.domainName)

		var pattern DomainNamePattern
		err := pattern.Parse(test.pattern)
		if assert.NoError(err, label) {
			match := pattern.Match(test.domainName)
			assert.Equal(test.match, match, label)
		}
	}
}

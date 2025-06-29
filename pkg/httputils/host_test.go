package httputils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseHostHeaderField(t *testing.T) {
	assert := assert.New(t)

	tests := []struct {
		field string
		host  string
	}{
		{"127.0.0.1", "127.0.0.1"},
		{"127.0.0.1:80", "127.0.0.1"},
		{"[::1]", "::1"},
		{"[::1]:80", "::1"},
		{"[2600:1406:bc00:53::b81e:94ce]", "2600:1406:bc00:53::b81e:94ce"},
		{"[2600:1406:bc00:53::b81e:94ce]:80", "2600:1406:bc00:53::b81e:94ce"},
		{"example.com", "example.com"},
		{"example.com:80", "example.com"},
	}

	for _, test := range tests {
		host, err := ParseHostHeaderField(test.field)
		if assert.NoError(err, test.field) {
			assert.Equal(test.host, host, test.field)
		}
	}

	invalidTests := []string{
		"",
		":",
		":80",
		"127.0.0.1:",
		"127",
		"[",
		"[::1",
		"[::1]:",
		"example.com:",
	}

	for _, field := range invalidTests {
		_, err := ParseHostHeaderField(field)
		assert.Error(err, field)
	}
}

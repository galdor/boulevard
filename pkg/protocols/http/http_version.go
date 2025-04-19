package http

import (
	"fmt"

	"go.n16f.net/bcl"
)

type HTTPVersion string

const (
	HTTPVersionHTTP10 HTTPVersion = "HTTP/1.0"
	HTTPVersionHTTP11 HTTPVersion = "HTTP/1.1"
	HTTPVersionHTTP2  HTTPVersion = "HTTP/2"
	HTTPVersionHTTP3  HTTPVersion = "HTTP/3"
)

var HTTPVersionStringsAny = []any{
	string(HTTPVersionHTTP10),
	string(HTTPVersionHTTP11),
	string(HTTPVersionHTTP2),
	string(HTTPVersionHTTP3),
}

func (v HTTPVersion) Match(major, minor int) (match bool) {
	switch v {
	case HTTPVersionHTTP10:
		match = major == 1 && minor == 0
	case HTTPVersionHTTP11:
		match = major == 1 && minor == 1
	case HTTPVersionHTTP2:
		match = major == 2
	case HTTPVersionHTTP3:
		match = major == 3
	default:
		panic(fmt.Sprintf("unhandled HTTP version %q", v))
	}

	return
}

func (v *HTTPVersion) Parse(s string) error {
	switch s {
	case "HTTP/1.0":
		*v = HTTPVersionHTTP10
	case "HTTP/1.1":
		*v = HTTPVersionHTTP11
	case "HTTP/2":
		*v = HTTPVersionHTTP2
	case "HTTP/3":
		*v = HTTPVersionHTTP3
	default:
		return fmt.Errorf("invalid HTTP version")
	}

	return nil
}

// bcl.ValueReader
func (v *HTTPVersion) ReadBCLValue(value *bcl.Value) error {
	var s string

	switch value.Type() {
	case bcl.ValueTypeString:
		s = value.Content.(bcl.String).String
	default:
		return bcl.NewValueTypeError(value, bcl.ValueTypeString)
	}

	return v.Parse(s)
}

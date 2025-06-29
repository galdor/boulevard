package httputils

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

func ParseHostHeaderField(s string) (string, error) {
	// The Host header field contains the host (hostname or IP address) the
	// client is targeting, which may be different from the host the client is
	// connecting to for servers supporting multiple hosts. For some (incredibly
	// bad) reason it can also contain a port number.
	//
	// The Go net/http standard package looks for the Host header field,
	// validates it and puts it in a specific field of the Request object. The
	// net.SplitHostPort is able to parse the construct but fails if the port
	// number is not set, and the error types are private so we cannot even
	// check for them.
	//
	// So we have to reimplement address parsing. As usual, the Go standard
	// library is terrible.
	//
	// See RFC 9110 7.2. Host and :authority

	if len(s) == 0 {
		return "", fmt.Errorf("empty value")
	}

	var host, portString string

	if s[0] == '[' {
		// IPv6 address
		before, after, found := strings.Cut(s[1:], "]")
		if !found {
			return "", fmt.Errorf("truncated IPv6 address")
		}

		host = before

		if len(host) == 0 {
			return "", fmt.Errorf("empty host")
		}

		addr := net.ParseIP(host)
		if addr == nil || addr.To16() == nil {
			return "", fmt.Errorf("invalid IPv6 address")
		}

		if len(after) > 0 {
			if after[0] != ':' {
				return "", fmt.Errorf("invalid character %q after IPv6 address",
					after[0])
			}

			if len(after) < 2 {
				return "", fmt.Errorf("truncated value")
			}
			portString = after[1:]
		}
	} else {
		// IPv4 address or hostname
		before, after, found := strings.Cut(s, ":")
		if found && after == "" {
			return "", fmt.Errorf("truncated value")
		}

		host = before

		if len(host) == 0 {
			return "", fmt.Errorf("empty host")
		}

		if host[0] >= '1' && host[0] <= '9' {
			// IPv4 address
			addr := net.ParseIP(host)
			if addr == nil || addr.To4() == nil {
				return "", fmt.Errorf("invalid IPv4 address")
			}
		}

		portString = after
	}

	if len(portString) > 0 {
		port, err := strconv.ParseInt(portString, 10, 64)
		if err != nil || port < 1 || port > 65535 {
			return "", fmt.Errorf("invalid port number %q", port)
		}
	}

	return host, nil
}

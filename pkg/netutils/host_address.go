package netutils

import (
	"fmt"
	"net"
	"strconv"

	"go.n16f.net/bcl"
)

// A representation of an host address (host and port) where we know if the host
// is either an hostname or address. This is used for load balancers where the
// way we resolve hostnames depends on the configuration.

type HostAddress struct {
	// One or the other
	Hostname string
	Address  net.IP

	Port int
}

func (ha *HostAddress) String() string {
	host := ha.Hostname
	if host == "" {
		host = ha.Address.String()
	}

	return net.JoinHostPort(host, strconv.Itoa(ha.Port))
}

func (ha *HostAddress) ReadBCLValue(v *bcl.Value) error {
	var s string

	switch v.Type() {
	case bcl.ValueTypeString:
		s = v.Content.(bcl.String).String
	default:
		return bcl.NewValueTypeError(v, bcl.ValueTypeString)
	}

	host, port, err := net.SplitHostPort(s)
	if err != nil {
		return fmt.Errorf("invalid format: %w", err)
	}

	if (host[0] >= '0' && host[0] <= '9') || host[0] == ':' {
		// IP address
		addr := net.ParseIP(host)
		if addr == nil {
			return fmt.Errorf("invalid IP address %q", host)
		}
		ha.Address = addr
	} else {
		// Hostname
		ha.Hostname = host
	}

	port64, err := strconv.ParseInt(port, 10, 64)
	if err != nil || port64 < 1 || port64 > 65535 {
		return fmt.Errorf("invalid port number %q", port)
	}
	ha.Port = int(port64)

	return nil
}

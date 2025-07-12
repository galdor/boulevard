package netutils

import (
	"fmt"
	"net"

	"go.n16f.net/bcl"
)

type Host struct {
	// One or the other
	Hostname string
	Address  net.IP
}

func (h *Host) ReadBCLValue(v *bcl.Value) error {
	var s string

	switch v.Type() {
	case bcl.ValueTypeString:
		s = v.Content.(bcl.String).String
	default:
		return bcl.NewValueTypeError(v, bcl.ValueTypeString)
	}

	if (s[0] >= '0' && s[0] <= '9') || s[0] == ':' {
		// IP address
		addr := net.ParseIP(s)
		if addr == nil {
			return fmt.Errorf("invalid IP address")
		}
		h.Address = addr
	} else {
		// Hostname
		h.Hostname = s
	}

	return nil
}

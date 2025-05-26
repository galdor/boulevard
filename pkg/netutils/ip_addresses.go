package netutils

import (
	"fmt"
	"net"

	"go.n16f.net/bcl"
)

type IPNetAddr net.IPNet

func (addr IPNetAddr) String() string {
	ipNet := net.IPNet(addr)
	return ipNet.String()
}

func (addr *IPNetAddr) Parse(s string) error {
	_, ipNetAddr, err := net.ParseCIDR(s)
	if err != nil {
		return fmt.Errorf("invalid format")
	}

	*addr = IPNetAddr(*ipNetAddr)

	return nil
}

func (addr *IPNetAddr) ReadBCLValue(v *bcl.Value) error {
	var s string

	switch v.Type() {
	case bcl.ValueTypeString:
		s = v.Content.(bcl.String).String
	default:
		return bcl.NewValueTypeError(v, bcl.ValueTypeString)
	}

	if err := addr.Parse(s); err != nil {
		return fmt.Errorf("invalid IP network address: %w", err)
	}

	return nil
}

type IPAddr net.IPNet

func (addr IPAddr) String() string {
	ipNet := net.IPNet(addr)
	return ipNet.String()
}

func (addr *IPAddr) Parse(s string) error {
	ipAddr := net.ParseIP(s)
	if ipAddr == nil {
		return fmt.Errorf("invalid format")
	}

	bitLen := len(ipAddr) * 8
	mask := net.CIDRMask(bitLen, bitLen)
	*addr = IPAddr(net.IPNet{IP: ipAddr, Mask: mask})

	return nil
}

func (addr *IPAddr) ReadBCLValue(v *bcl.Value) error {
	var s string

	switch v.Type() {
	case bcl.ValueTypeString:
		s = v.Content.(bcl.String).String
	default:
		return bcl.NewValueTypeError(v, bcl.ValueTypeString)
	}

	if err := addr.Parse(s); err != nil {
		return fmt.Errorf("invalid IP address: %w", err)
	}

	return nil
}

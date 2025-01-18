package netutils

import (
	"errors"
	"fmt"
	"net"
	"strconv"
)

func ConnectionRemoteAddress(conn net.Conn) (net.IP, int, error) {
	netAddr := conn.RemoteAddr()
	if netAddr == nil {
		return nil, 0, fmt.Errorf("missing remote address")
	}

	addr, port, err := ParseNumericAddress(netAddr.String())
	if err != nil {
		return nil, 0, fmt.Errorf("invalid remote address %q: %w",
			netAddr.String(), err)
	}

	return addr, port, nil
}

func ParseNumericAddress(s string) (net.IP, int, error) {
	host, port, err := net.SplitHostPort(s)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid format")
	}

	ipAddress := net.ParseIP(host)
	if ipAddress == nil {
		return nil, 0, fmt.Errorf("invalid IP address %q", host)
	}

	portNumber, err := strconv.ParseInt(port, 10, 64)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return nil, 0, fmt.Errorf("invalid port number %q", port)
	}

	return ipAddress, int(portNumber), nil
}

func FormatNumericAddress(addr net.IP, portNumber int) string {
	if len(addr) == net.IPv6len {
		return fmt.Sprintf("[%v]:%d", addr, portNumber)
	} else {
		return fmt.Sprintf("%v:%d", addr, portNumber)
	}
}

func ValidateBCLAddress(v any) error {
	_, portString, err := net.SplitHostPort(v.(string))
	if err != nil {
		var addrErr *net.AddrError
		var msg string

		if errors.As(err, &addrErr) {
			msg = addrErr.Err
		} else {
			msg = err.Error()
		}

		return fmt.Errorf("invalid address: %s", msg)
	}

	if portString == "" {
		return fmt.Errorf("invalid address: empty port number")
	}

	port, err := strconv.ParseInt(portString, 10, 64)
	if err != nil || port < 1 || port >= 65535 {
		return fmt.Errorf("invalid address: invalid port number %q", portString)
	}

	return nil
}

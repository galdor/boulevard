package netutils

import (
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

package netutils

import (
	"errors"
	"fmt"
	"net"
	"strconv"
)

func UnwrapOpError(err error, name string) error {
	var opErr *net.OpError

	if errors.As(err, &opErr) && opErr.Op == name {
		return opErr.Err
	}

	return err
}

func ConnectionRemoteAddress(conn net.Conn) (net.IP, int, error) {
	netAddr := conn.RemoteAddr()
	if netAddr == nil {
		return nil, 0, fmt.Errorf("missing remote address")
	}

	host, port, err := net.SplitHostPort(netAddr.String())
	if err != nil {
		return nil, 0, fmt.Errorf("invalid remote address %q", netAddr)
	}

	ipAddress := net.ParseIP(host)
	if ipAddress == nil {
		return nil, 0, fmt.Errorf("invalid remote IP address %q", host)
	}

	portNumber, err := strconv.ParseInt(port, 10, 64)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return nil, 0, fmt.Errorf("invalid port %q", port)
	}

	return ipAddress, int(portNumber), nil
}

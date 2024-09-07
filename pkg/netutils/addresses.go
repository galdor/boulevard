package netutils

import (
	"fmt"
	"net"
	"strconv"
)

func ParseAddress(s string) (string, int, error) {
	host, portString, err := net.SplitHostPort(s)
	if err != nil {
		return "", 0, fmt.Errorf("cannot parse address: %w", err)
	}

	i64, err := strconv.ParseInt(portString, 10, 64)
	if err != nil || i64 < 1 || i64 > 65535 {
		return "", 0, fmt.Errorf("cannot parse port number")
	}

	return host, int(i64), nil
}

package netutils

import (
	"crypto/tls"
	"net"
)

func TCPListen(address string, tlsCfg *TLSCfg) (net.Listener, error) {
	l, err := net.Listen("tcp", address)
	if err != nil {
		return nil, err
	}

	if tlsCfg == nil {
		return l, nil
	}

	return tls.NewListener(l, tlsCfg.NetTLSConfig()), nil
}

package netutils

import (
	"errors"
	"io"
	"net"
)

func IsConnectionClosedError(err error) bool {
	return errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed)
}

func UnwrapOpError(err error, name string) error {
	var opErr *net.OpError

	if errors.As(err, &opErr) && opErr.Op == name {
		return opErr.Err
	}

	return err
}

package netutils

import (
	"errors"
	"net"
)

func UnwrapOpError(err error, name string) error {
	var opErr *net.OpError

	if errors.As(err, &opErr) && opErr.Op == name {
		return opErr.Err
	}

	return err
}

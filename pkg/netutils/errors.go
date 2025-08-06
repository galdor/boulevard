package netutils

import (
	"crypto/tls"
	"errors"
	"io"
	"net"
	"os"
	"syscall"
)

func IsSilentIOError(err error) bool {
	// In a lot of situations network IO operations fail either because the
	// client unexpectedly closing the connection or because something out of
	// our control happened to it. We do not want to fill logs with these
	// errors, so we do our best to identify them.

	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}

	if errors.Is(err, net.ErrClosed) {
		return true
	}

	var syscallErr *os.SyscallError
	if errors.As(err, &syscallErr) {
		errno := syscallErr.Err

		switch errno {
		case syscall.ECONNRESET, syscall.EPIPE:
			return true
		}
	}

	return false
}

func IsTLSError(err error) bool {
	var recordHeaderErr tls.RecordHeaderError

	if errors.As(err, &recordHeaderErr) {
		return true
	}

	return false
}

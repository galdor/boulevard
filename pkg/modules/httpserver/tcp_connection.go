package httpserver

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"syscall"

	"go.n16f.net/boulevard/pkg/netutils"
	"go.n16f.net/log"
)

const (
	TCPConnectionReadBufferSize  = 1024
	TCPConnectionWriteBufferSize = 1024
)

type TCPConnection struct {
	Module   *Module
	Listener *Listener
	Log      *log.Logger

	conn         net.Conn
	upstreamConn net.Conn
	mutex        sync.Mutex
}

func (c *TCPConnection) Close() {
	c.Log.Debug(1, "closing connection")

	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}

	if c.upstreamConn != nil {
		c.upstreamConn.Close()
		c.upstreamConn = nil
	}
}

func (c *TCPConnection) abort(err error) {
	if isSilentIOError(err) {
		c.Log.Debug(1, "%v", err)
	} else {
		c.Log.Error("%v", err)
	}

	c.Close()
	c.Listener.unregisterTCPConnection(c)
}

func (c *TCPConnection) read() {
	defer c.Listener.wg.Done()

	buf := make([]byte, TCPConnectionReadBufferSize)

	for {
		n, err := c.conn.Read(buf)
		if err != nil {
			err = netutils.UnwrapOpError(err, "read")
			c.abort(fmt.Errorf("cannot read connection: %w", err))
			return
		}

		if _, err := c.upstreamConn.Write(buf[:n]); err != nil {
			err = netutils.UnwrapOpError(err, "write")
			c.abort(fmt.Errorf("cannot write proxy connection: %w", err))
			return
		}
	}
}

func (c *TCPConnection) write() {
	defer c.Listener.wg.Done()

	buf := make([]byte, TCPConnectionWriteBufferSize)

	for {
		n, err := c.upstreamConn.Read(buf)
		if err != nil {
			err = netutils.UnwrapOpError(err, "read")
			c.abort(fmt.Errorf("cannot read proxy connection: %w", err))
			return
		}

		if _, err := c.conn.Write(buf[:n]); err != nil {
			err = netutils.UnwrapOpError(err, "write")
			c.abort(fmt.Errorf("cannot write connection: %w", err))
			return
		}
	}
}

func isSilentIOError(err error) bool {
	if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
		return true
	}

	var syscallErr *os.SyscallError
	if errors.As(err, &syscallErr) {
		errno := syscallErr.Err

		if errno == syscall.EPIPE {
			return true
		}
	}

	return false
}

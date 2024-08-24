package tcpserver

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"syscall"

	"go.n16f.net/boulevard/pkg/netutils"
	"go.n16f.net/log"
)

type Connection struct {
	Module   *Module
	Listener *Listener
	Log      *log.Logger

	conn      net.Conn
	proxyConn net.Conn
}

func (c *Connection) Close() {
	c.Log.Debug(1, "closing connection")

	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}

	if c.proxyConn != nil {
		c.proxyConn.Close()
		c.proxyConn = nil
	}
}

func (c *Connection) abort(err error) {
	if isSilentIOError(err) {
		c.Log.Debug(1, "%v", err)
	} else {
		c.Log.Error("%v", err)
	}

	c.Close()
	c.Listener.unregisterConnection(c)
}

func (c *Connection) read() {
	defer c.Listener.wg.Done()

	buf := make([]byte, c.Module.Cfg.ReadBufferSize)

	for {
		n, err := c.conn.Read(buf)
		if err != nil {
			err = netutils.UnwrapOpError(err, "read")
			c.abort(fmt.Errorf("cannot read connection: %w", err))
			return
		}

		if _, err := c.proxyConn.Write(buf[:n]); err != nil {
			err = netutils.UnwrapOpError(err, "write")
			c.abort(fmt.Errorf("cannot write proxy connection: %w", err))
			return
		}
	}
}

func (c *Connection) write() {
	defer c.Listener.wg.Done()

	buf := make([]byte, c.Module.Cfg.WriteBufferSize)

	for {
		n, err := c.proxyConn.Read(buf)
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

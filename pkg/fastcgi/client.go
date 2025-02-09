package fastcgi

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"syscall"

	"go.n16f.net/log"
)

var (
	ErrConnectionLimitReached = errors.New("connection limit reached")
)

type ClientCfg struct {
	Log *log.Logger

	Address        string
	MaxConnections *int
}

type Client struct {
	Cfg *ClientCfg
	Log *log.Logger

	usedConns     map[*Connection]struct{}
	idleConns     map[*Connection]struct{}
	connMutex     sync.Mutex
	releasedConns chan *Connection
}

func NewClient(cfg *ClientCfg) (*Client, error) {
	c := Client{
		Cfg: cfg,
		Log: cfg.Log,

		usedConns:     make(map[*Connection]struct{}),
		idleConns:     make(map[*Connection]struct{}),
		releasedConns: make(chan *Connection),
	}

	return &c, nil
}

func (c *Client) Close() {
	c.connMutex.Lock()
	defer c.connMutex.Unlock()

	for conn := range c.usedConns {
		conn.Close()
	}

	for conn := range c.idleConns {
		conn.Close()
	}

	c.usedConns = nil
	c.idleConns = nil

	close(c.releasedConns)
}

func (c *Client) FetchValues(ctx context.Context) (NameValuePairs, error) {
	conn, err := c.acquireConnection(ctx)
	if err != nil {
		return nil, err
	}
	defer c.releaseConnection(conn)

	return conn.Values(), nil
}

func (c *Client) SendRequest(ctx context.Context, role Role, params NameValuePairs, stdin, data io.Reader, stdout io.Writer, stderr *bytes.Buffer) (*Header, error) {
	var retry bool

sendRequest:
	header, err := c.sendRequest(ctx, role, params, stdin, data, stdout, stderr)
	if err != nil {
		// We do not want to fail if the connection to the upstream server was
		// severed: most of the time, we can just reconnect and it will work
		// just fine. Of course we only try once: if it is down, it is down.

		if !retry && isConnectionClosedWriteError(err) {
			retry = true
			goto sendRequest
		}

		return nil, err
	}

	return header, nil
}

func (c *Client) sendRequest(ctx context.Context, role Role, params NameValuePairs, stdin, data io.Reader, stdout io.Writer, stderr *bytes.Buffer) (*Header, error) {
	conn, err := c.acquireConnection(ctx)
	if err != nil {
		return nil, err
	}
	defer c.releaseConnection(conn)

	header, err := conn.SendRequest(ctx, role, params, stdin, data, stdout,
		stderr)
	if err != nil {
		conn.Close()
		return nil, err
	}

	return header, nil
}

func (c *Client) acquireConnection(ctx context.Context) (*Connection, error) {
	var conn *Connection

	c.connMutex.Lock()

	for conn2 := range c.idleConns {
		conn = conn2
		break
	}

	if conn != nil {
		delete(c.idleConns, conn)
		c.usedConns[conn] = struct{}{}

		c.connMutex.Unlock()
		return conn, nil
	}

	select {
	case conn := <-c.releasedConns:
		c.connMutex.Unlock()
		return conn, nil
	case <-ctx.Done():
	default:
	}

	if maxConns := c.Cfg.MaxConnections; maxConns != nil {
		if len(c.usedConns)+len(c.idleConns) > *maxConns {
			c.connMutex.Unlock()
			return nil, ErrConnectionLimitReached
		}
	}

	c.connMutex.Unlock()

	conn, err := NewConnection(ctx, c)
	if err != nil {
		return nil, fmt.Errorf("cannot create connection: %w", err)
	}

	c.connMutex.Lock()
	c.usedConns[conn] = struct{}{}
	c.connMutex.Unlock()

	return conn, nil
}

func (c *Client) releaseConnection(conn *Connection) {
	// Note how we take care to release the mutex before writing the chan: if we
	// did not, a consumer trying to acquire a connection would never be able to
	// read since they need the lock to do so.

	c.connMutex.Lock()
	delete(c.usedConns, conn)
	c.connMutex.Unlock()

	if conn.conn == nil {
		return
	}

	select {
	case c.releasedConns <- conn:
		return
	default:
	}

	c.connMutex.Lock()
	c.idleConns[conn] = struct{}{}
	c.connMutex.Unlock()
}

func isConnectionClosedWriteError(err error) bool {
	var syscallErr *os.SyscallError

	if errors.As(err, &syscallErr) {
		if errno, ok := syscallErr.Err.(syscall.Errno); ok {
			if errno == syscall.EPIPE {
				return true
			}
		}
	}

	return false
}

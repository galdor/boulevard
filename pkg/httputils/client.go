package httputils

import (
	"bufio"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrClientStopping        = errors.New("client stopping")
	ErrNoConnectionAvailable = errors.New("no connection available")
)

type ClientCfg struct {
	Scheme string
	Host   string

	TLS *tls.Config

	MaxConnections               int
	ConnectionTimeout            time.Duration
	ConnectionAcquisitionTimeout time.Duration
}

type Client struct {
	Cfg ClientCfg
	tls bool

	nbConns       atomic.Int32
	idleConns     []*ClientConn
	idleConnMutex sync.Mutex

	releasedConns chan *ClientConn

	stopChan chan struct{}
}

type ClientConn struct {
	Conn   net.Conn
	reader *bufio.Reader
}

func (c *ClientConn) Close() {
	if c.Conn != nil {
		c.Conn.Close()
		c.Conn = nil
	}
}

func NewClient(cfg ClientCfg) (*Client, error) {
	c := Client{
		Cfg: cfg,

		releasedConns: make(chan *ClientConn),

		stopChan: make(chan struct{}),
	}

	switch strings.ToLower(cfg.Scheme) {
	case "http":
		c.tls = false
	case "https":
		c.tls = true
	default:
		return nil, fmt.Errorf("unsupported scheme %q", cfg.Scheme)
	}

	return &c, nil
}

func (c *Client) Stop() {
	close(c.stopChan)

	for _, c := range c.idleConns {
		c.Close()
	}

	close(c.releasedConns)

	c.nbConns.Store(0)
}

func (c *Client) WithRoundTrip(req *http.Request, fn func(*ClientConn, *http.Response) (bool, error)) error {
	return c.withConn(func(conn *ClientConn) (bool, error) {
		if err := req.Write(conn.Conn); err != nil {
			return false, fmt.Errorf("cannot write request: %w", err)
		}

		res, err := http.ReadResponse(conn.reader, req)
		if err != nil {
			return false, fmt.Errorf("cannot read response: %w", err)
		}
		defer res.Body.Close()

		return fn(conn, res)
	})
}

func (c *Client) withConn(fn func(*ClientConn) (bool, error)) error {
	var conn *ClientConn

	c.idleConnMutex.Lock()
	if len(c.idleConns) > 0 {
		conn = c.idleConns[0]
		c.idleConns = c.idleConns[1:]
	}
	c.idleConnMutex.Unlock()

	if conn == nil {
		// Of course this is not perfectly accurate since multiple goroutines
		// can connect concurrently, but the last thing we want is to lock the
		// whole client during a connection.

		nbConns := int(c.nbConns.Load())

		if nbConns < c.Cfg.MaxConnections {
			var err error
			conn, err = c.connect()
			if err != nil {
				return fmt.Errorf("cannot connect to %q: %w", c.Cfg.Host, err)
			}

			c.nbConns.Add(1)
		} else {
			select {
			case conn = <-c.releasedConns:

			case <-time.After(c.Cfg.ConnectionAcquisitionTimeout):
				return ErrNoConnectionAvailable

			case <-c.stopChan:
				return ErrClientStopping
			}
		}

	}

	hijack, err := fn(conn)
	if err != nil {
		conn.Close()
		c.nbConns.Add(-1)
		return err
	}

	if !hijack {
		select {
		case c.releasedConns <- conn:
		default:
			c.idleConnMutex.Lock()
			c.idleConns = append(c.idleConns, conn)
			c.idleConnMutex.Unlock()
		}
	}

	return nil
}

func (c *Client) connect() (*ClientConn, error) {
	timeout := c.Cfg.ConnectionTimeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	dialer := net.Dialer{
		Timeout: timeout,
	}

	var conn net.Conn
	var err error

	if c.tls {
		conn, err = tls.DialWithDialer(&dialer, "tcp", c.Cfg.Host, c.Cfg.TLS)
	} else {
		conn, err = dialer.Dial("tcp", c.Cfg.Host)
	}
	if err != nil {
		return nil, err
	}

	cc := ClientConn{
		Conn:   conn,
		reader: bufio.NewReader(conn),
	}

	return &cc, nil
}

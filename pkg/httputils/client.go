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
	IdleConnectionTimeout        time.Duration
}

type Client struct {
	Cfg ClientCfg
	tls bool

	nbConns       atomic.Int32
	idleConns     []*ClientConn
	idleConnMutex sync.Mutex

	idleConnTimer *time.Timer

	releasedConns chan *ClientConn

	stopChan chan struct{}
	wg       sync.WaitGroup
}

type ClientConn struct {
	Conn   net.Conn
	reader *bufio.Reader

	lastActivity atomic.Pointer[time.Time]
}

func (c *ClientConn) Close() {
	if c.Conn != nil {
		c.Conn.Close()
		c.Conn = nil
	}
}

func (c *ClientConn) SendRequest(req *http.Request) (*http.Response, error) {
	now := time.Now()
	c.lastActivity.Store(&now)

	if err := req.Write(c.Conn); err != nil {
		return nil, fmt.Errorf("cannot write request: %w", err)
	}

	res, err := http.ReadResponse(c.reader, req)
	if err != nil {
		return nil, fmt.Errorf("cannot read response: %w", err)
	}

	return res, nil
}

func NewClient(cfg ClientCfg) (*Client, error) {
	if cfg.MaxConnections == 0 {
		cfg.MaxConnections = 10
	}

	if cfg.ConnectionTimeout == 0 {
		cfg.ConnectionTimeout = 10 * time.Second
	}

	if cfg.ConnectionAcquisitionTimeout == 0 {
		cfg.ConnectionAcquisitionTimeout = 5 * time.Second
	}

	if cfg.IdleConnectionTimeout == 0 {
		cfg.IdleConnectionTimeout = 10 * time.Minute
	}

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

	c.wg.Add(1)
	go c.watchConnections()

	return &c, nil
}

func (c *Client) Stop() {
	close(c.stopChan)
	c.wg.Wait()

	for _, c := range c.idleConns {
		c.Close()
	}

	close(c.releasedConns)

	c.nbConns.Store(0)
}

func (c *Client) AcquireConn() (*ClientConn, error) {
	var conn *ClientConn

	c.idleConnMutex.Lock()
	if len(c.idleConns) > 0 {
		conn = c.idleConns[0]
		c.idleConns = c.idleConns[1:]
		c.idleConnMutex.Unlock()
		return conn, nil
	}
	c.idleConnMutex.Unlock()

	// Of course this is not perfectly accurate since multiple goroutines
	// can connect concurrently, but the last thing we want is to lock the
	// whole client during a connection.

	nbConns := int(c.nbConns.Load())

	if nbConns < c.Cfg.MaxConnections {
		var err error
		conn, err = c.connect()
		if err != nil {
			return nil, fmt.Errorf("cannot connect to %q: %w", c.Cfg.Host, err)
		}

		c.nbConns.Add(1)

		return conn, nil
	}

	select {
	case conn = <-c.releasedConns:
		return conn, nil

	case <-time.After(c.Cfg.ConnectionAcquisitionTimeout):
		return nil, ErrNoConnectionAvailable

	case <-c.stopChan:
		return nil, ErrClientStopping
	}
}

func (c *Client) HijackConn(conn *ClientConn) {
	c.nbConns.Add(-1)
}

func (c *Client) ReleaseConn(conn *ClientConn) {
	if conn.Conn == nil {
		c.nbConns.Add(-1)
		return
	}

	select {
	case c.releasedConns <- conn:
	default:
		c.idleConnMutex.Lock()
		c.idleConns = append(c.idleConns, conn)
		c.idleConnMutex.Unlock()
	}
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

func (c *Client) watchConnections() {
	defer c.wg.Done()

	c.idleConnTimer = time.NewTimer(c.Cfg.IdleConnectionTimeout)
	defer c.idleConnTimer.Stop()

	for {
		select {
		case <-c.stopChan:
			return

		case <-c.idleConnTimer.C:
			c.closeIdleConnections()
		}
	}
}

func (c *Client) closeIdleConnections() {
	now := time.Now()

	c.idleConnMutex.Lock()
	defer c.idleConnMutex.Unlock()

	var idleConns []*ClientConn
	nextTimeout := now.Add(c.Cfg.IdleConnectionTimeout)

	for _, conn := range c.idleConns {
		lastActivity := conn.lastActivity.Load()
		connTimeout := lastActivity.Add(c.Cfg.IdleConnectionTimeout)

		if connTimeout.Before(now) {
			conn.Close()
		} else {
			idleConns = append(idleConns, conn)

			if connTimeout.Before(nextTimeout) {
				nextTimeout = connTimeout
			}
		}
	}

	c.idleConns = idleConns
	c.idleConnTimer.Reset(nextTimeout.Sub(now))
}

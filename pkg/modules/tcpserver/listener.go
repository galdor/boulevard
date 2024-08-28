package tcpserver

import (
	"context"
	"fmt"
	"net"
	"sync"

	"go.n16f.net/boulevard/pkg/netutils"
	"go.n16f.net/log"
)

type Listener struct {
	Module      *Module
	TCPListener *netutils.TCPListener

	connections      map[*Connection]struct{}
	connectionsMutex sync.Mutex

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewListener(mod *Module, cfg netutils.TCPListenerCfg) (*Listener, error) {
	cfg.Logger = mod.Log
	cfg.ACMEClient = mod.acmeClient

	tcpListener, err := netutils.NewTCPListener(cfg)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	l := Listener{
		Module:      mod,
		TCPListener: tcpListener,

		connections: make(map[*Connection]struct{}),

		ctx:    ctx,
		cancel: cancel,
	}

	return &l, nil
}

func (l *Listener) Start() error {
	if err := l.TCPListener.Start(); err != nil {
		return err
	}

	l.wg.Add(1)
	go l.listen()

	return nil
}

func (l *Listener) Stop() {
	l.TCPListener.Listener.Close() // interrupt Accept

	l.cancel()

	l.connectionsMutex.Lock()
	for connection := range l.connections {
		connection.Close() // interrupt Read and Write
		delete(l.connections, connection)
	}
	l.connectionsMutex.Unlock()

	l.wg.Wait()
}

func (l *Listener) CountConnections() int64 {
	l.connectionsMutex.Lock()
	n := len(l.connections)
	l.connectionsMutex.Unlock()

	return int64(n)
}

func (l *Listener) fatal(format string, args ...any) {
	err := fmt.Errorf(format, args...)

	select {
	case l.Module.errChan <- err:
	case <-l.ctx.Done():
	}
}

func (l *Listener) listen() {
	defer l.wg.Done()

	for {
		conn, err := l.TCPListener.Accept()
		if err != nil {
			l.fatal("cannot accept connection: %w", err)
			return
		}

		l.handleConnection(conn)
	}
}

func (l *Listener) handleConnection(conn net.Conn) {
	addr, port, err := netutils.ConnectionRemoteAddress(conn)
	if err != nil {
		l.Module.Log.Error("cannot identify connection remote address: %v", err)
		return
	}

	proxyCfg := l.Module.Cfg.Proxy
	proxyConn, err := net.Dial("tcp", proxyCfg.Address)
	if err != nil {
		err = netutils.UnwrapOpError(err, "accept")
		l.Module.Log.Error("cannot connect to %q: %v", proxyCfg.Address, err)
		conn.Close()
		return
	}

	logData := log.Data{
		"connection": fmt.Sprintf("%s:%d", addr, port),
	}

	logger := l.Module.Log.Child("", logData)

	c := Connection{
		Module:   l.Module,
		Listener: l,
		Log:      logger,

		conn:      conn,
		proxyConn: proxyConn,
	}

	c.Listener.registerConnection(&c)

	l.wg.Add(2)
	go c.read()
	go c.write()
}

func (l *Listener) registerConnection(c *Connection) {
	l.connectionsMutex.Lock()
	l.connections[c] = struct{}{}
	l.connectionsMutex.Unlock()
}

func (l *Listener) unregisterConnection(c *Connection) {
	l.connectionsMutex.Lock()
	delete(l.connections, c)
	l.connectionsMutex.Unlock()
}

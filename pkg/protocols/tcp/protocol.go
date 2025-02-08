package tcp

import (
	"net"
	"sync"

	"go.n16f.net/bcl"
	"go.n16f.net/boulevard/pkg/boulevard"
	"go.n16f.net/boulevard/pkg/netutils"
	"go.n16f.net/log"
)

func ProtocolInfo() *boulevard.ProtocolInfo {
	return &boulevard.ProtocolInfo{
		Name:           "tcp",
		InstantiateCfg: NewProtocolCfg,
		Instantiate:    NewProtocol,
	}
}

const (
	DefaultReadBufferSize  = 16 * 1024
	DefaultWriteBufferSize = 16 * 1024
)

type ProtocolCfg struct {
	ReadBufferSize  int
	WriteBufferSize int

	ReverseProxy ReverseProxyAction
}

func NewProtocolCfg() boulevard.ProtocolCfg {
	return &ProtocolCfg{}
}

func (cfg *ProtocolCfg) ReadBCLElement(block *bcl.Element) error {
	cfg.ReadBufferSize = DefaultReadBufferSize
	block.MaybeEntryValue("read_buffer_size",
		bcl.WithValueValidation(&cfg.ReadBufferSize,
			bcl.ValidatePositiveInteger))

	cfg.WriteBufferSize = DefaultWriteBufferSize
	block.MaybeEntryValue("write_buffer_size",
		bcl.WithValueValidation(&cfg.WriteBufferSize,
			bcl.ValidatePositiveInteger))

	block.Element("reverse_proxy", &cfg.ReverseProxy)

	return nil
}

type ReverseProxyAction struct {
	Address string
}

func (cfg *ReverseProxyAction) ReadBCLElement(elt *bcl.Element) error {
	if elt.IsBlock() {
		elt.EntryValue("address",
			bcl.WithValueValidation(&cfg.Address, netutils.ValidateBCLAddress))
	} else {
		elt.Value(
			bcl.WithValueValidation(&cfg.Address, netutils.ValidateBCLAddress))
	}

	return nil
}

type Protocol struct {
	Cfg    *ProtocolCfg
	Log    *log.Logger
	Server *boulevard.Server

	connections     map[*Connection]struct{}
	connectionMutex sync.Mutex

	wg sync.WaitGroup
}

func NewProtocol() boulevard.Protocol {
	return &Protocol{}
}

func (p *Protocol) Start(server *boulevard.Server) error {
	p.Cfg = server.Cfg.ProtocolCfg.(*ProtocolCfg)
	p.Log = server.Log
	p.Server = server

	p.connections = make(map[*Connection]struct{})

	for _, l := range server.Listeners {
		go p.listen(l)
	}

	return nil
}

func (p *Protocol) Stop() {
	p.connectionMutex.Lock()
	for conn := range p.connections {
		conn.Close() // interrupt Read and/or Write
	}
	p.connectionMutex.Unlock()

	p.wg.Wait()
}

func (p *Protocol) listen(l *boulevard.Listener) {
	for {
		conn, err := l.Listener.Accept()
		if err != nil {
			p.Server.Fatal("cannot accept connection: %w", err)
			return
		}

		p.handleConnection(l, conn)
	}
}

func (p *Protocol) handleConnection(l *boulevard.Listener, conn net.Conn) {
	addr, _, err := netutils.ConnectionRemoteAddress(conn)
	if err != nil {
		p.Log.Error("cannot identify connection remote address: %v", err)
		return
	}

	cfg := p.Cfg.ReverseProxy
	upstreamConn, err := net.Dial("tcp", cfg.Address)
	if err != nil {
		err = netutils.UnwrapOpError(err, "dial")
		p.Log.Error("cannot connect to %q: %v", cfg.Address, err)
		conn.Close()
		return
	}

	logData := log.Data{
		"address": addr.String(),
	}

	c := Connection{
		Protocol: p,
		Listener: l,
		Log:      p.Log.Child("", logData),

		conn:         conn,
		upstreamConn: upstreamConn,
	}

	p.registerConnection(&c)

	p.wg.Add(2)
	go c.read()
	go c.write()
}

func (p *Protocol) registerConnection(c *Connection) {
	p.connectionMutex.Lock()
	p.connections[c] = struct{}{}
	p.connectionMutex.Unlock()
}

func (p *Protocol) unregisterConnection(c *Connection) {
	p.connectionMutex.Lock()
	delete(p.connections, c)
	p.connectionMutex.Unlock()
}

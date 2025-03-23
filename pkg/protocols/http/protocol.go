package http

import (
	"fmt"
	"sync"
	"sync/atomic"

	"go.n16f.net/bcl"
	"go.n16f.net/boulevard/pkg/boulevard"
	"go.n16f.net/log"
)

func ProtocolInfo() *boulevard.ProtocolInfo {
	return &boulevard.ProtocolInfo{
		Name:           "http",
		InstantiateCfg: NewProtocolCfg,
		Instantiate:    NewProtocol,
	}
}

type ProtocolCfg struct {
	Handlers     []*HandlerCfg
	AccessLogger *AccessLoggerCfg

	DebugLogVariables bool
}

func NewProtocolCfg() boulevard.ProtocolCfg {
	return &ProtocolCfg{}
}

func (cfg *ProtocolCfg) ReadBCLElement(block *bcl.Element) error {
	block.Blocks("handler", &cfg.Handlers)
	block.MaybeBlock("access_logs", &cfg.AccessLogger)

	block.MaybeEntryValues("debug_log_variables", &cfg.DebugLogVariables)

	return nil
}

type Protocol struct {
	Cfg    *ProtocolCfg
	Log    *log.Logger
	Server *boulevard.Server

	nbConnections      atomic.Int64
	tcpConnections     map[*TCPConnection]struct{}
	tcpConnectionMutex sync.Mutex

	vars         map[string]string
	accessLogger *AccessLogger
	handlers     []*Handler
	servers      []*Server

	wg sync.WaitGroup
}

func NewProtocol() boulevard.Protocol {
	return &Protocol{}
}

func (p *Protocol) Start(server *boulevard.Server) error {
	p.Cfg = server.Cfg.ProtocolCfg.(*ProtocolCfg)
	p.Log = server.Log
	p.Server = server

	p.tcpConnections = make(map[*TCPConnection]struct{})

	p.vars = make(map[string]string)
	p.vars["server.name"] = server.Cfg.Name

	if logCfg := p.Cfg.AccessLogger; logCfg != nil {
		log, err := NewAccessLogger(logCfg, p.vars)
		if err != nil {
			return fmt.Errorf("cannot create access logger: %w", err)
		}

		p.accessLogger = log
	}

	p.handlers = make([]*Handler, len(p.Cfg.Handlers))
	for i, cfg := range p.Cfg.Handlers {
		handler, err := StartHandler(p, cfg)
		if err != nil {
			for j := range i {
				p.handlers[j].Stop()
			}

			return fmt.Errorf("cannot create handler: %w", err)
		}

		p.handlers[i] = handler
	}

	p.servers = make([]*Server, len(server.Listeners))
	for i, l := range server.Listeners {
		server, err := StartServer(p, l)
		if err != nil {
			for j := range i {
				p.servers[j].Stop()
			}

			return fmt.Errorf("cannot create server: %w", err)
		}

		p.servers[i] = server
	}

	return nil
}

func (p *Protocol) Stop() {
	for _, server := range p.servers {
		server.Stop()
	}

	for _, handler := range p.handlers {
		handler.Stop()
	}

	p.tcpConnectionMutex.Lock()
	for conn := range p.tcpConnections {
		conn.Close()
	}
	p.tcpConnectionMutex.Unlock()

	p.nbConnections.Store(0)

	if p.accessLogger != nil {
		p.accessLogger.Close()
	}

	p.wg.Wait()
}

func (p *Protocol) findHandler(ctx *RequestContext) *Handler {
	var find func([]*Handler, *Handler) *Handler

	find = func(handlers []*Handler, lastMatch *Handler) *Handler {
		for _, h := range handlers {
			if h.matchRequest(ctx) {
				if h2 := find(h.Handlers, h); h2 != nil {
					return h2
				}

				return h
			}
		}

		return lastMatch
	}

	return find(p.handlers, nil)
}

func (p *Protocol) registerTCPConnection(c *TCPConnection) {
	p.tcpConnectionMutex.Lock()
	p.tcpConnections[c] = struct{}{}
	p.tcpConnectionMutex.Unlock()
}

func (p *Protocol) unregisterTCPConnection(c *TCPConnection) {
	p.tcpConnectionMutex.Lock()
	delete(p.tcpConnections, c)
	p.tcpConnectionMutex.Unlock()
}

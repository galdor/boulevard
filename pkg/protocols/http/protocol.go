package http

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

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

type TLSHandling string

const (
	TLSHandlingAccept   TLSHandling = "accept"
	TLSHandlingReject   TLSHandling = "reject"
	TLSHandlingRequire  TLSHandling = "require"
	TLSHandlingRedirect TLSHandling = "redirect"
)

type ProtocolCfg struct {
	Handlers     []*HandlerCfg
	AccessLogger *AccessLoggerCfg
	TLSHandling  TLSHandling
	HSTS         bool

	DebugLogVariables bool
	LogGoServerErrors bool // [1]
	UnencryptedHTTP2  bool

	// [1] Note that http/server logs TLS errors as part of HTTP server logs;
	// this is why there is no log_tls_errors setting as for TCP servers.
}

func NewProtocolCfg() boulevard.ProtocolCfg {
	return &ProtocolCfg{}
}

func (cfg *ProtocolCfg) ReadBCLElement(block *bcl.Element) error {
	block.Blocks("handler", &cfg.Handlers)
	block.MaybeBlock("access_logs", &cfg.AccessLogger)

	cfg.TLSHandling = TLSHandlingAccept
	if entry := block.FindEntry("tls"); entry != nil {
		entry.CheckValueOneOf(0, "accept", "reject", "require", "redirect")

		var s string
		entry.Values(&s)
		cfg.TLSHandling = TLSHandling(s)
	}

	block.MaybeEntryValues("hsts", &cfg.HSTS)

	block.MaybeEntryValues("debug_log_variables", &cfg.DebugLogVariables)
	block.MaybeEntryValues("log_go_server_errors", &cfg.LogGoServerErrors)
	block.MaybeEntryValues("unencrypted_http2", &cfg.UnencryptedHTTP2)

	return nil
}

type Protocol struct {
	Cfg    *ProtocolCfg
	Log    *log.Logger
	Server *boulevard.Server

	nbConnections      atomic.Int64
	tcpConnections     map[*TCPConnection]struct{}
	tcpConnectionMutex sync.Mutex

	vars               map[string]string
	accessLogger       *AccessLogger
	handlers           []*Handler
	servers            []*Server
	defaultTLSListener *boulevard.Listener

	stopChan chan struct{}
	wg       sync.WaitGroup
}

func NewProtocol() boulevard.Protocol {
	return &Protocol{
		stopChan: make(chan struct{}),
	}
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

		if l.Cfg.TLS != nil && p.defaultTLSListener == nil {
			p.defaultTLSListener = l
		}
	}

	p.wg.Add(1)
	go p.rateLimiterGC()

	return nil
}

func (p *Protocol) Stop() {
	close(p.stopChan)

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

func (p *Protocol) RotateLogFiles() {
	reopen := func(accessLogger *AccessLogger) {
		if accessLogger == nil {
			return
		}

		p.Log.Info("rotating %q", accessLogger.FilePath())

		if err := accessLogger.Reopen(); err != nil {
			p.Log.Info("cannot reopen %q: %v", accessLogger.FilePath(), err)
		}
	}

	reopen(p.accessLogger)

	var rotateHandlerLoggers func(handlers []*Handler)
	rotateHandlerLoggers = func(handlers []*Handler) {
		for _, h := range handlers {
			reopen(h.AccessLogger)
			rotateHandlerLoggers(h.Handlers)
		}
	}

	rotateHandlerLoggers(p.handlers)
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

func (p *Protocol) rateLimiterGC() {
	defer p.wg.Done()

	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.gcRateLimiters()

		case <-p.stopChan:
			return
		}
	}
}

func (p *Protocol) gcRateLimiters() {
	var gc func([]*Handler)
	gc = func(handlers []*Handler) {
		for _, handler := range handlers {
			if rl := handler.RequestRateLimiter; rl != nil {
				rl.GC()
			}

			gc(handler.Handlers)
		}
	}

	gc(p.handlers)
}

package httpserver

import (
	"fmt"

	"go.n16f.net/bcl"
	"go.n16f.net/boulevard/pkg/boulevard"
	"go.n16f.net/boulevard/pkg/netutils"
	"go.n16f.net/log"
)

func ModuleInfo() *boulevard.ModuleInfo {
	return &boulevard.ModuleInfo{
		Type:           "http_server",
		InstantiateCfg: NewModuleCfg,
		Instantiate:    NewModule,
	}
}

type ModuleCfg struct {
	Listeners    []*netutils.TCPListenerCfg
	Handlers     []*HandlerCfg
	AccessLogger *AccessLoggerCfg
}

func (cfg *ModuleCfg) Init(block *bcl.Element) {
	// TODO Validate minimum number of blocks 1
	for _, block := range block.Blocks("listener") {
		var lcfg netutils.TCPListenerCfg
		lcfg.Init(block)

		cfg.Listeners = append(cfg.Listeners, &lcfg)
	}

	for _, block := range block.Blocks("handler") {
		var hcfg HandlerCfg
		hcfg.Init(block)

		cfg.Handlers = append(cfg.Handlers, &hcfg)
	}

	if block := block.MaybeBlock("access_logs"); block != nil {
		cfg.AccessLogger = new(AccessLoggerCfg)
		cfg.AccessLogger.Init(block)
	}
}

func NewModuleCfg() boulevard.ModuleCfg {
	return &ModuleCfg{}
}

type Module struct {
	Cfg  *ModuleCfg
	Log  *log.Logger
	Data *boulevard.ModuleData

	Vars map[string]string

	listeners    []*Listener
	handlers     []*Handler
	accessLogger *AccessLogger
}

func NewModule() boulevard.Module {
	return &Module{}
}

func (mod *Module) Start(modCfg boulevard.ModuleCfg, modData *boulevard.ModuleData) error {
	mod.Cfg = modCfg.(*ModuleCfg)
	mod.Log = modData.Logger
	mod.Data = modData

	mod.Vars = make(map[string]string)
	mod.Vars["module.name"] = modData.Name

	mod.handlers = make([]*Handler, len(mod.Cfg.Handlers))
	for i, cfg := range mod.Cfg.Handlers {
		handler, err := NewHandler(mod, cfg)
		if err != nil {
			return fmt.Errorf("cannot create handler: %w", err)
		}

		if err := handler.Start(); err != nil {
			for j := range i {
				mod.handlers[j].Stop()
			}

			return fmt.Errorf("cannot start handler: %w", err)
		}

		mod.handlers[i] = handler
	}

	mod.listeners = make([]*Listener, len(mod.Cfg.Listeners))
	for i, cfg := range mod.Cfg.Listeners {
		if cfg.TLS != nil {
			cfg.TLS.CertificateName = fmt.Sprintf("%s-%d", modData.Name, i)
		}

		listener, err := NewListener(mod, *cfg)
		if err != nil {
			return fmt.Errorf("cannot create listener: %w", err)
		}

		if err := listener.Start(); err != nil {
			for j := range i {
				mod.listeners[j].Stop()
			}

			return fmt.Errorf("cannot start listener: %w", err)
		}

		mod.listeners[i] = listener
	}

	if logCfg := mod.Cfg.AccessLogger; logCfg != nil {
		log, err := NewAccessLogger(logCfg, mod.Vars)
		if err != nil {
			return fmt.Errorf("cannot create access logger: %w", err)
		}

		mod.accessLogger = log
	}

	return nil
}

func (mod *Module) Stop() {
	for _, listener := range mod.listeners {
		listener.Stop()
	}

	for _, handler := range mod.handlers {
		handler.Stop()
	}

	if mod.accessLogger != nil {
		mod.accessLogger.Close()
	}
}

func (mod *Module) findHandler(ctx *RequestContext) *Handler {
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

	return find(mod.handlers, nil)
}

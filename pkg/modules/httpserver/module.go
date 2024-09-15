package httpserver

import (
	"fmt"

	"go.n16f.net/boulevard/pkg/boulevard"
	"go.n16f.net/boulevard/pkg/netutils"
	"go.n16f.net/ejson"
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
	Listeners []*netutils.TCPListenerCfg `json:"listeners"`
	Handlers  []*HandlerCfg              `json:"handlers,omitempty"`
}

func (cfg *ModuleCfg) ValidateJSON(v *ejson.Validator) {
	v.CheckArrayNotEmpty("listeners", cfg.Listeners)
	v.CheckObjectArray("listeners", cfg.Listeners)

	v.CheckObjectArray("handlers", cfg.Handlers)
}

func NewModuleCfg() boulevard.ModuleCfg {
	return &ModuleCfg{}
}

type Module struct {
	Cfg  *ModuleCfg
	Log  *log.Logger
	Data *boulevard.ModuleData

	listeners []*Listener
	handlers  []*Handler
}

func NewModule() boulevard.Module {
	return &Module{}
}

func (mod *Module) Start(modCfg boulevard.ModuleCfg, modData *boulevard.ModuleData) error {
	mod.Cfg = modCfg.(*ModuleCfg)
	mod.Log = modData.Logger
	mod.Data = modData

	mod.handlers = make([]*Handler, len(mod.Cfg.Handlers))
	for i, cfg := range mod.Cfg.Handlers {
		handler, err := NewHandler(mod, *cfg)
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

	return nil
}

func (mod *Module) Stop() {
	for _, listener := range mod.listeners {
		listener.Stop()
	}

	for _, handler := range mod.handlers {
		handler.Stop()
	}
}

func (mod *Module) findHandler(ctx *RequestContext) *Handler {
	for _, h := range mod.handlers {
		if h.matchRequest(ctx) {
			return h
		}
	}

	return nil
}

package httpserver

import (
	"fmt"

	"go.n16f.net/acme"
	"go.n16f.net/boulevard/pkg/boulevard"
	"go.n16f.net/ejson"
	"go.n16f.net/log"
)

func ModuleInfo() *boulevard.ModuleInfo {
	return &boulevard.ModuleInfo{
		Name:           "http-server",
		InstantiateCfg: NewModuleCfg,
		Instantiate:    NewModule,
	}
}

type ModuleCfg struct {
	Listeners []*ListenerCfg `json:"listeners"`
	Handlers  []*Handler     `json:"handlers,omitempty"`
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
	Cfg *ModuleCfg
	Log *log.Logger

	errChan    chan<- error
	acmeClient *acme.Client
	listeners  []*Listener
}

func NewModule() boulevard.Module {
	return &Module{}
}

func (mod *Module) Start(modCfg boulevard.ModuleCfg, modData *boulevard.ModuleData) error {
	mod.Cfg = modCfg.(*ModuleCfg)
	mod.Log = modData.Logger

	mod.errChan = modData.ErrChan
	mod.acmeClient = modData.ACMEClient

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
}

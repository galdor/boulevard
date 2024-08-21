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

	acmeClient *acme.Client
	listeners  []*Listener
}

func NewModule(modCfg boulevard.ModuleCfg, modData boulevard.ModuleData) (boulevard.Module, error) {
	cfg := modCfg.(*ModuleCfg)

	mod := Module{
		Cfg: cfg,

		acmeClient: modData.ACMEClient,
	}

	mod.listeners = make([]*Listener, len(cfg.Listeners))
	for i, lCfg := range cfg.Listeners {
		if lCfg.TLS != nil {
			lCfg.TLS.CertificateName = fmt.Sprintf("%s-%d", modData.Name, i)
		}

		listener, err := NewListener(&mod, *lCfg)
		if err != nil {
			return nil, fmt.Errorf("cannot create listener: %w", err)
		}

		mod.listeners[i] = listener
	}

	return &mod, nil
}

func (mod *Module) Start(logger *log.Logger) error {
	mod.Log = logger

	for i, l := range mod.listeners {
		if err := l.Start(); err != nil {
			for j := range i {
				mod.listeners[j].Stop()
			}

			return fmt.Errorf("cannot start listener: %w", err)
		}
	}

	return nil
}

func (mod *Module) Stop() {
	for _, l := range mod.listeners {
		l.Stop()
	}
}

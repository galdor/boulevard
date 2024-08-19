package httpserver

import (
	"fmt"

	"go.n16f.net/boulevard/pkg/boulevard"
	"go.n16f.net/ejson"
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
}

func (cfg *ModuleCfg) ValidateJSON(v *ejson.Validator) {
	v.CheckArrayNotEmpty("listeners", cfg.Listeners)
	v.CheckObjectArray("listeners", cfg.Listeners)
}

func NewModuleCfg() boulevard.ModuleCfg {
	return &ModuleCfg{}
}

type Module struct {
	Cfg *ModuleCfg

	Listeners []*Listener
}

func NewModule(modCfg boulevard.ModuleCfg) (boulevard.Module, error) {
	cfg := modCfg.(*ModuleCfg)

	mod := Module{
		Cfg: cfg,
	}

	mod.Listeners = make([]*Listener, len(cfg.Listeners))
	for i, lCfg := range cfg.Listeners {
		mod.Listeners[i] = NewListener(*lCfg)
	}

	return &mod, nil
}

func (mod *Module) Start() error {
	for i, l := range mod.Listeners {
		if err := l.Start(); err != nil {
			for j := range i {
				mod.Listeners[j].Stop()
			}

			return fmt.Errorf("cannot start listener: %w", err)
		}
	}

	return nil
}

func (mod *Module) Stop() {
	for _, l := range mod.Listeners {
		l.Stop()
	}
}

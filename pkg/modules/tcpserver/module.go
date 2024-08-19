package tcpserver

import (
	"go.n16f.net/boulevard/pkg/boulevard"
	"go.n16f.net/ejson"
)

func ModuleInfo() *boulevard.ModuleInfo {
	return &boulevard.ModuleInfo{
		Name:           "tcp-server",
		InstantiateCfg: NewModuleCfg,
		Instantiate:    NewModule,
	}
}

type ModuleCfg struct {
	// TODO
}

func (cfg *ModuleCfg) ValidateJSON(v *ejson.Validator) {
}

func NewModuleCfg() boulevard.ModuleCfg {
	return &ModuleCfg{}
}

type Module struct {
	// TODO
}

func NewModule(modCfg boulevard.ModuleCfg) (boulevard.Module, error) {
	//cfg := modCfg.(*ModuleCfg)
	mod := Module{}
	return &mod, nil
}

func (mod *Module) Start() error {
	return nil
}

func (mod *Module) Stop() {
}

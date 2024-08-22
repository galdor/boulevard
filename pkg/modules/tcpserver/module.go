package tcpserver

import (
	"go.n16f.net/boulevard/pkg/boulevard"
	"go.n16f.net/ejson"
	"go.n16f.net/log"
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
	Cfg *ModuleCfg
	Log *log.Logger
}

func NewModule() boulevard.Module {
	return &Module{}
}

func (mod *Module) Start(modCfg boulevard.ModuleCfg, modData *boulevard.ModuleData) error {
	mod.Cfg = modCfg.(*ModuleCfg)
	mod.Log = modData.Logger

	// TODO

	return nil
}

func (mod *Module) Stop() {
	// TODO
}

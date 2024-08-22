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
	Cfg  *ModuleCfg
	Data boulevard.ModuleData
	Log  *log.Logger

	errChan chan<- error
}

func NewModule(modCfg boulevard.ModuleCfg, modData boulevard.ModuleData) (boulevard.Module, error) {
	cfg := modCfg.(*ModuleCfg)

	mod := Module{
		Cfg: cfg,
	}

	return &mod, nil
}

func (mod *Module) Start(logger *log.Logger, errChan chan<- error) error {
	mod.Log = logger
	mod.errChan = errChan

	// TODO

	return nil
}

func (mod *Module) Stop() {
	// TODO
}

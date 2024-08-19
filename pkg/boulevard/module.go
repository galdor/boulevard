package boulevard

import (
	"go.n16f.net/ejson"
	"go.n16f.net/log"
)

type ModuleInfo struct {
	Name           string
	InstantiateCfg func() ModuleCfg
	Instantiate    func(ModuleCfg) (Module, error)
}

type ModuleCfg interface {
	ejson.Validatable
}

type Module interface {
	Start(*log.Logger) error
	Stop()
}

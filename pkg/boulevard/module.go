package boulevard

import (
	"go.n16f.net/acme"
	"go.n16f.net/ejson"
	"go.n16f.net/log"
)

type ModuleInfo struct {
	Name           string
	InstantiateCfg func() ModuleCfg
	Instantiate    func(ModuleCfg, ModuleData) (Module, error)
}

type ModuleCfg interface {
	ejson.Validatable
}

type ModuleData struct {
	Name       string
	ACMEClient *acme.Client
}

type Module interface {
	Start(*log.Logger, chan<- error) error
	Stop()
}

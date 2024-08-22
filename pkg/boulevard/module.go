package boulevard

import (
	"go.n16f.net/acme"
	"go.n16f.net/ejson"
	"go.n16f.net/log"
)

type ModuleInfo struct {
	Name           string
	InstantiateCfg func() ModuleCfg
	Instantiate    func() Module
}

type ModuleCfg interface {
	ejson.Validatable
}

type ModuleData struct {
	Name string

	Logger  *log.Logger
	ErrChan chan<- error

	ACMEClient *acme.Client
}

type Module interface {
	Start(ModuleCfg, *ModuleData) error
	Stop()
}

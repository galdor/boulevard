package boulevard

import (
	"go.n16f.net/acme"
	"go.n16f.net/ejson"
	"go.n16f.net/log"
)

type ModuleInfo struct {
	Type           string
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

	ModuleStatus   func(string) *ModuleStatus
	ModuleStatuses func() []*ModuleStatus
}

type Module interface {
	Start(ModuleCfg, *ModuleData) error
	Stop()

	StatusData() any
}

type ModuleStatus struct {
	Name string      `json:"name"`
	Info *ModuleInfo `json:"-"`
	Cfg  ModuleCfg   `json:"-"`
	Data any         `json:"data"`
}

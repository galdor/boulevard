package boulevard

import (
	"go.n16f.net/acme"
	"go.n16f.net/bcl"
	"go.n16f.net/log"
)

type ModuleInfo struct {
	Type           string
	InstantiateCfg func() ModuleCfg
	Instantiate    func() Module
}

type ModuleCfg interface {
	bcl.ElementReader
}

type ModuleData struct {
	Id   string
	Name string

	Logger  *log.Logger
	ErrChan chan<- error

	ACMEClient *acme.Client

	ModuleStatus   func(string) *ModuleStatus
	ModuleStatuses func() []*ModuleStatus

	BoulevardBuildId string
}

type Module interface {
	Start(ModuleCfg, *ModuleData) error
	Stop()

	StatusData() any
}

type ModuleStatus struct {
	Name string      `json:"name"`
	Info *ModuleInfo `json:"-"`
	Data any         `json:"data"`
}

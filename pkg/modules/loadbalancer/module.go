package loadbalancer

import (
	"fmt"

	"go.n16f.net/bcl"
	"go.n16f.net/boulevard/pkg/boulevard"
	"go.n16f.net/log"
)

func ModuleInfo() *boulevard.ModuleInfo {
	return &boulevard.ModuleInfo{
		Type:           "load_balancer",
		InstantiateCfg: NewModuleCfg,
		Instantiate:    NewModule,
	}
}

type ModuleCfg struct {
	Servers []string
}

func (cfg *ModuleCfg) ReadBCLElement(block *bcl.Element) error {
	for _, entry := range block.FindEntries("server") {
		var address string
		entry.Value(&address)
		cfg.Servers = append(cfg.Servers, address)
	}

	return nil
}

func NewModuleCfg() boulevard.ModuleCfg {
	return &ModuleCfg{}
}

type Module struct {
	Cfg  *ModuleCfg
	Log  *log.Logger
	Data *boulevard.ModuleData

	servers []*Server
}

func NewModule() boulevard.Module {
	return &Module{}
}

func (mod *Module) Start(modCfg boulevard.ModuleCfg, modData *boulevard.ModuleData) error {
	mod.Cfg = modCfg.(*ModuleCfg)
	mod.Log = modData.Logger
	mod.Data = modData

	mod.servers = make([]*Server, len(mod.Cfg.Servers))
	for i, address := range mod.Cfg.Servers {
		server, err := NewServer(mod, address)
		if err != nil {
			for j := range i {
				mod.servers[j].Stop()
			}

			return fmt.Errorf("cannot create server: %w", err)
		}

		mod.servers[i] = server
	}

	return nil
}

func (mod *Module) Stop() {
	for _, server := range mod.servers {
		server.Stop()
	}
}

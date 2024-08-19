package service

import (
	"encoding/json"
	"fmt"

	"go.n16f.net/boulevard/pkg/boulevard"
	"go.n16f.net/ejson"
	"go.n16f.net/log"
)

type ModuleCfg struct {
	Module string          `json:"module"`
	Name   string          `json:"name"`
	Data   json.RawMessage `json:"data"`
}

func (cfg *ModuleCfg) ValidateJSON(v *ejson.Validator) {
	// TODO
}

func (s *Service) initModules() error {
	for _, i := range s.Cfg.ModuleInfo {
		s.moduleInfo[i.Name] = i
	}

	for _, modCfg := range s.Cfg.Modules {
		info := s.moduleInfo[modCfg.Module]
		if info == nil {
			return fmt.Errorf("unknown module type %q", modCfg.Module)
		}

		if _, found := s.modules[modCfg.Name]; found {
			return fmt.Errorf("duplicate module name %q", modCfg.Name)
		}

		cfg := info.InstantiateCfg()
		if err := ejson.Unmarshal(modCfg.Data, cfg); err != nil {
			return fmt.Errorf("invalid configuration for module %q: %w",
				modCfg.Name, err)
		}

		mod, err := info.Instantiate(cfg)
		if err != nil {
			return fmt.Errorf("cannot instantiate module %q: %w",
				modCfg.Name, err)
		}

		s.modules[modCfg.Name] = mod
	}

	return nil
}

func (s *Service) startModules() error {
	var startedMods []boulevard.Module

	for name, mod := range s.modules {
		s.Log.Debug(1, "starting module %q", name)

		logger := s.Log.Child("module", log.Data{"module": name})

		if err := mod.Start(logger); err != nil {
			for _, startedMod := range startedMods {
				startedMod.Stop()
			}

			return fmt.Errorf("cannot start module %q: %w", name, err)
		}

		startedMods = append(startedMods, mod)
	}

	return nil
}

func (s *Service) stopModules() {
	for name, mod := range s.modules {
		s.Log.Debug(1, "stopping module %q", name)

		mod.Stop()
	}
}

package service

import (
	"encoding/json"
	"fmt"
	"time"

	"go.n16f.net/boulevard/pkg/boulevard"
	"go.n16f.net/ejson"
	"go.n16f.net/log"
)

type Module struct {
	Info   *boulevard.ModuleInfo
	Cfg    *ModuleCfg
	Data   *boulevard.ModuleData
	Logger *log.Logger
	Module boulevard.Module
}

type ModuleCfg struct {
	Module string          `json:"module"`
	Name   string          `json:"name"`
	Data   json.RawMessage `json:"data"`
}

func (cfg *ModuleCfg) ValidateJSON(v *ejson.Validator) {
	v.CheckStringNotEmpty("name", cfg.Name)
}

func (s *Service) startModules() error {
	for _, i := range s.Cfg.ModuleInfo {
		s.moduleInfo[i.Name] = i
	}

	s.modulesMutex.Lock()
	defer s.modulesMutex.Unlock()

	var startedMods []string

	for _, modCfg := range s.Cfg.Modules {
		if err := s.startModule(modCfg); err != nil {
			for _, startedMod := range startedMods {
				s.stopModule(startedMod)
			}

			return fmt.Errorf("cannot start module %q: %w", modCfg.Name, err)
		}
	}

	return nil
}

func (s *Service) startModule(modCfg *ModuleCfg) error {
	if _, found := s.modules[modCfg.Name]; found {
		return fmt.Errorf("duplicate module name %q", modCfg.Name)
	}

	info, found := s.moduleInfo[modCfg.Module]
	if !found {
		return fmt.Errorf("unknown module type %q", modCfg.Module)
	}

	cfg := info.InstantiateCfg()
	if err := ejson.Unmarshal(modCfg.Data, cfg); err != nil {
		return fmt.Errorf("cannot parse configuration of module %q: %w",
			modCfg.Name, err)
	}

	s.Log.Debug(1, "starting %s module %q", modCfg.Module, modCfg.Name)

	logger := s.Log.Child("module", log.Data{"module": modCfg.Name})

	errChan := make(chan error)
	go func() {
		if err := <-errChan; err != nil {
			s.handleModuleError(modCfg.Name, err)
		}
	}()

	data := boulevard.ModuleData{
		Name: modCfg.Name,

		Logger:  logger,
		ErrChan: errChan,

		ACMEClient: s.acmeClient,

		ModuleStatus:   s.moduleStatus,
		ModuleStatuses: s.moduleStatuses,
	}

	mod := Module{
		Info:   info,
		Cfg:    modCfg,
		Data:   &data,
		Logger: logger,
		Module: info.Instantiate(),
	}

	if err := mod.Module.Start(cfg, &data); err != nil {
		close(errChan)
		return err
	}

	s.modules[modCfg.Name] = &mod

	return nil
}

func (s *Service) stopModules() {
	s.modulesMutex.Lock()
	defer s.modulesMutex.Unlock()

	for name := range s.modules {
		s.stopModule(name)
	}
}

func (s *Service) stopModule(name string) {
	mod, found := s.modules[name]
	if !found {
		return
	}

	s.Log.Debug(1, "stopping module %q", name)

	mod.Module.Stop()
	delete(s.modules, name)

	close(mod.Data.ErrChan)
}

func (s *Service) handleModuleError(name string, err error) {
	select {
	case <-s.stopChan:
		return
	default:
	}

	s.modulesMutex.Lock()
	defer s.modulesMutex.Unlock()

	mod := s.modules[name]

	mod.Logger.Error("fatal: %v", err)

	s.stopModule(name)

	go func() {
		for {
			select {
			case <-time.After(5 * time.Second):
			case <-s.stopChan:
				return
			}

			func() {
				s.modulesMutex.Lock()
				defer s.modulesMutex.Unlock()

				if err := s.startModule(mod.Cfg); err != nil {
					mod.Logger.Error("cannot restart module %q: %v",
						mod.Cfg.Name, err)
				}
			}()
		}
	}()
}

func (s *Service) moduleStatus(name string) *boulevard.ModuleStatus {
	s.modulesMutex.Lock()
	defer s.modulesMutex.Unlock()

	mod := s.modules[name]
	if mod == nil {
		return nil
	}

	status := boulevard.ModuleStatus{
		Name: name,
		Cfg:  mod.Cfg,
		Data: mod.Module.StatusData(),
	}

	return &status
}

func (s *Service) moduleStatuses() []*boulevard.ModuleStatus {
	s.modulesMutex.Lock()
	defer s.modulesMutex.Unlock()

	var statuses []*boulevard.ModuleStatus

	for name, mod := range s.modules {
		status := boulevard.ModuleStatus{
			Name: name,
			Info: mod.Info,
			Data: mod.Module.StatusData(),
		}

		statuses = append(statuses, &status)
	}

	return statuses
}

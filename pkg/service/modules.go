package service

import (
	"fmt"
	"time"

	"go.n16f.net/boulevard/pkg/boulevard"
	modhttpserver "go.n16f.net/boulevard/pkg/modules/httpserver"
	modtcpserver "go.n16f.net/boulevard/pkg/modules/tcpserver"
	"go.n16f.net/log"
)

var DefaultModules = []*boulevard.ModuleInfo{
	modhttpserver.ModuleInfo(),
	modtcpserver.ModuleInfo(),
}

type Module struct {
	Info   *boulevard.ModuleInfo
	Cfg    *ModuleCfg
	Data   *boulevard.ModuleData
	Logger *log.Logger
	Module boulevard.Module
}

type ModuleCfg struct {
	Info *boulevard.ModuleInfo
	Name string
	Cfg  boulevard.ModuleCfg
}

func (cfg *ModuleCfg) Id() string {
	return cfg.Info.Type + "." + cfg.Name
}

func (s *Service) startModules() error {
	for _, info := range s.Cfg.ModuleInfo {
		s.moduleInfo[info.Type] = info
	}

	s.modulesMutex.Lock()
	defer s.modulesMutex.Unlock()

	var startedMods []string

	for _, cfg := range s.Cfg.Modules {
		if err := s.startModule(cfg); err != nil {
			for _, startedMod := range startedMods {
				s.stopModule(startedMod)
			}

			return fmt.Errorf("cannot start module %s: %w", cfg.Id(), err)
		}
	}

	return nil
}

func (s *Service) startModule(cfg *ModuleCfg) error {
	id := cfg.Id()

	if _, found := s.modules[id]; found {
		return fmt.Errorf("duplicate module %s", id)
	}

	s.Log.Debug(1, "starting module %s", id)

	logger := s.Log.Child("module", log.Data{"module": id})

	errChan := make(chan error)
	go func() {
		if err := <-errChan; err != nil {
			s.handleModuleError(id, err)
		}
	}()

	modData := boulevard.ModuleData{
		Id:   id,
		Name: cfg.Name,

		Logger:  logger,
		ErrChan: errChan,

		ACMEClient: s.acmeClient,

		ModuleStatus:   s.moduleStatus,
		ModuleStatuses: s.moduleStatuses,

		BoulevardBuildId: s.Cfg.BuildId,
	}

	mod := Module{
		Info:   cfg.Info,
		Cfg:    cfg,
		Data:   &modData,
		Logger: logger,
		Module: cfg.Info.Instantiate(),
	}

	if err := mod.Module.Start(cfg.Cfg, &modData); err != nil {
		close(errChan)
		return err
	}

	s.modules[id] = &mod

	return nil
}

func (s *Service) stopModules() {
	s.modulesMutex.Lock()
	defer s.modulesMutex.Unlock()

	for id := range s.modules {
		s.stopModule(id)
	}
}

func (s *Service) stopModule(id string) {
	mod, found := s.modules[id]
	if !found {
		return
	}

	s.Log.Debug(1, "stopping module %s", id)

	mod.Module.Stop()
	delete(s.modules, id)

	close(mod.Data.ErrChan)
}

func (s *Service) handleModuleError(id string, err error) {
	select {
	case <-s.stopChan:
		return
	default:
	}

	s.modulesMutex.Lock()
	defer s.modulesMutex.Unlock()

	mod := s.modules[id]

	mod.Logger.Error("fatal: %v", err)

	s.stopModule(id)

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
					mod.Logger.Error("cannot restart module %s: %v", id, err)
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

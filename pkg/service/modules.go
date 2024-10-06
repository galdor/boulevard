package service

import (
	"encoding/json"
	"fmt"
	"time"

	"go.n16f.net/boulevard/pkg/boulevard"
	modhttpserver "go.n16f.net/boulevard/pkg/modules/httpserver"
	modtcpserver "go.n16f.net/boulevard/pkg/modules/tcpserver"
	"go.n16f.net/ejson"
	"go.n16f.net/log"
	"go.n16f.net/program"
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
	Module string `json:"module"`

	HTTPServer json.RawMessage `json:"http_server,omitempty"`
	TCPServer  json.RawMessage `json:"tcp_server,omitempty"`
}

func (cfg *ModuleCfg) ValidateJSON(v *ejson.Validator) {
	v.CheckStringNotEmpty("module", cfg.Module)

	modFields := []json.RawMessage{
		cfg.HTTPServer,
		cfg.TCPServer,
	}

	nbMods := 0
	for _, modField := range modFields {
		if modField != nil {
			nbMods++
		}
	}

	if nbMods == 0 {
		v.AddError(nil, "invalid_configuration",
			"missing module configuration")
	} else if nbMods > 1 {
		v.AddError(nil, "invalid_configuration",
			"cannot provide multiple module configurations")
	}
}

func (cfg *ModuleCfg) ModuleTypeAndData() (string, []byte) {
	var t string
	var data []byte

	switch {
	case cfg.HTTPServer != nil:
		t, data = "http_server", cfg.HTTPServer
	case cfg.TCPServer != nil:
		t, data = "tcp_server", cfg.TCPServer
	default:
		program.Panic("missing module configuration in %#v", cfg)
	}

	return t, data
}

func (s *Service) startModules() error {
	for _, i := range s.Cfg.ModuleInfo {
		s.moduleInfo[i.Type] = i
	}

	s.modulesMutex.Lock()
	defer s.modulesMutex.Unlock()

	var startedMods []string

	for _, modCfg := range s.Cfg.Modules {
		if err := s.startModule(modCfg); err != nil {
			for _, startedMod := range startedMods {
				s.stopModule(startedMod)
			}

			return fmt.Errorf("cannot start module %q: %w", modCfg.Module, err)
		}
	}

	return nil
}

func (s *Service) startModule(modCfg *ModuleCfg) error {
	modName := modCfg.Module

	if _, found := s.modules[modName]; found {
		return fmt.Errorf("duplicate module name %q", modName)
	}

	modType, cfgData := modCfg.ModuleTypeAndData()

	info, found := s.moduleInfo[modType]
	if !found {
		return fmt.Errorf("unknown module type %q", modType)
	}

	cfg := info.InstantiateCfg()
	if err := ejson.Unmarshal(cfgData, cfg); err != nil {
		return fmt.Errorf("cannot parse configuration: %w", err)
	}

	s.Log.Debug(1, "starting %s module %q", modType, modName)

	logger := s.Log.Child("module", log.Data{"module": modName})

	errChan := make(chan error)
	go func() {
		if err := <-errChan; err != nil {
			s.handleModuleError(modName, err)
		}
	}()

	modData := boulevard.ModuleData{
		Name: modName,

		Logger:  logger,
		ErrChan: errChan,

		ACMEClient: s.acmeClient,

		ModuleStatus:   s.moduleStatus,
		ModuleStatuses: s.moduleStatuses,

		BoulevardBuildId: s.Cfg.BuildId,
	}

	mod := Module{
		Info:   info,
		Cfg:    modCfg,
		Data:   &modData,
		Logger: logger,
		Module: info.Instantiate(),
	}

	if err := mod.Module.Start(cfg, &modData); err != nil {
		close(errChan)
		return err
	}

	s.modules[modName] = &mod

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
					mod.Logger.Error("cannot restart module %q: %v", name, err)
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

package service

import (
	"fmt"
	"os"
	"path"

	"go.n16f.net/bcl"
	"go.n16f.net/boulevard/pkg/boulevard"
	"go.n16f.net/log"
)

type ServiceCfg struct {
	// Provided by the caller of NewService
	BuildId    string
	ModuleInfo []*boulevard.ModuleInfo

	// Populated by ServiceCfg.Load
	Logger       *log.LoggerCfg
	ACME         *ACMECfg
	ControlAPI   *ControlAPICfg
	PProfAddress string
	Modules      []*ModuleCfg
}

func (cfg *ServiceCfg) Load(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("cannot read %q: %w", filePath, err)
	}

	doc, err := bcl.Parse(data, path.Base(filePath))
	if err != nil {
		return fmt.Errorf("cannot parse %q: %w", filePath, err)
	}

	cfg.initLogger(doc)
	cfg.initACME(doc)
	cfg.initControlAPI(doc)
	cfg.initPProf(doc)
	cfg.initModules(doc, cfg.ModuleInfo)

	if err := doc.ValidationErrors(); err != nil {
		return fmt.Errorf("invalid configuration:\n%w", err)
	}

	return nil
}

func (cfg *ServiceCfg) initLogger(doc *bcl.Document) {
	block := doc.MaybeBlock("logger")
	if block == nil {
		return
	}

	var loggerCfg log.LoggerCfg

	block.CheckBlocksMaybeOneOf("terminal", "json")

	if block := block.MaybeBlock("terminal"); block != nil {
		loggerCfg.BackendType = log.BackendTypeTerminal

		var backendCfg log.TerminalBackendCfg
		block.MaybeEntryValue("color", &backendCfg.Color)

		loggerCfg.TerminalBackend = &backendCfg
	}

	if block := block.MaybeBlock("json"); block != nil {
		loggerCfg.BackendType = log.BackendTypeJSON

		var backendCfg log.JSONBackendCfg
		block.MaybeEntryValue("timestamp_key", &backendCfg.TimestampKey)
		block.MaybeEntryValue("timestamp_layout", &backendCfg.TimestampLayout)
		block.MaybeEntryValue("domain_key", &backendCfg.DomainKey)
		block.MaybeEntryValue("level_key", &backendCfg.LevelKey)
		block.MaybeEntryValue("message_key", &backendCfg.MessageKey)
		block.MaybeEntryValue("data_key", &backendCfg.DataKey)

		loggerCfg.JSONBackend = &backendCfg
	}

	block.MaybeEntryValue("debug_level", &loggerCfg.DebugLevel)

	cfg.Logger = &loggerCfg
}

func (cfg *ServiceCfg) initACME(doc *bcl.Document) {
	block := doc.MaybeBlock("acme")
	if block == nil {
		return
	}

	cfg.ACME = new(ACMECfg)
	cfg.ACME.Init(block)
}

func (cfg *ServiceCfg) initControlAPI(doc *bcl.Document) {
	block := doc.MaybeBlock("control_api")
	if block == nil {
		return
	}

	cfg.ControlAPI = new(ControlAPICfg)
	cfg.ControlAPI.Init(block)
}

func (cfg *ServiceCfg) initPProf(doc *bcl.Document) {
	block := doc.MaybeBlock("pprof")
	if block == nil {
		return
	}

	block.EntryValue("address", &cfg.PProfAddress)
}

func (cfg *ServiceCfg) initModules(doc *bcl.Document, modInfo []*boulevard.ModuleInfo) {
	for _, info := range modInfo {
		for _, block := range doc.Blocks(info.Type) {
			modCfg := info.InstantiateCfg()
			modCfg.Init(block)

			modCfg2 := ModuleCfg{
				Info: info,
				Name: block.BlockName(),
				Cfg:  modCfg,
			}

			cfg.Modules = append(cfg.Modules, &modCfg2)
		}
	}
}

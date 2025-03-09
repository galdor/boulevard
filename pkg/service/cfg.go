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
	BuildId      string
	ProtocolInfo []*boulevard.ProtocolInfo

	// Set by ServiceCfg.Load
	Logger       *log.LoggerCfg
	ACME         *ACMECfg
	ControlAPI   *ControlAPICfg
	PProfAddress string
	Servers      []*boulevard.ServerCfg
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

	doc.TopLevel.MaybeBlock("acme", &cfg.ACME)
	doc.TopLevel.MaybeBlock("control_api", &cfg.ControlAPI)

	cfg.initLogger(doc)
	cfg.initPProf(doc)
	cfg.initServers(doc, cfg.ProtocolInfo)

	if err := doc.ValidationErrors(); err != nil {
		return fmt.Errorf("invalid configuration:\n%w", err)
	}

	return nil
}

func (cfg *ServiceCfg) initLogger(doc *bcl.Document) {
	block := doc.FindBlock("logs")
	if block == nil {
		return
	}

	var loggerCfg log.LoggerCfg
	var hasBackend bool

	block.CheckBlocksMaybeOneOf("terminal", "json")

	if block := block.FindBlock("terminal"); block != nil {
		loggerCfg.BackendType = log.BackendTypeTerminal

		var backendCfg log.TerminalBackendCfg
		block.MaybeEntryValue("color", &backendCfg.Color)

		loggerCfg.TerminalBackend = &backendCfg
		hasBackend = true
	}

	if block := block.FindBlock("json"); block != nil {
		loggerCfg.BackendType = log.BackendTypeJSON

		var backendCfg log.JSONBackendCfg
		block.MaybeEntryValue("timestamp_key", &backendCfg.TimestampKey)
		block.MaybeEntryValue("timestamp_layout", &backendCfg.TimestampLayout)
		block.MaybeEntryValue("domain_key", &backendCfg.DomainKey)
		block.MaybeEntryValue("level_key", &backendCfg.LevelKey)
		block.MaybeEntryValue("message_key", &backendCfg.MessageKey)
		block.MaybeEntryValue("data_key", &backendCfg.DataKey)

		loggerCfg.JSONBackend = &backendCfg
		hasBackend = true
	}

	if !hasBackend {
		loggerCfg.BackendType = log.BackendTypeTerminal
		loggerCfg.TerminalBackend = &log.TerminalBackendCfg{}
	}

	block.MaybeEntryValue("debug_level", &loggerCfg.DebugLevel)

	cfg.Logger = &loggerCfg
}

func (cfg *ServiceCfg) initPProf(doc *bcl.Document) {
	block := doc.FindBlock("pprof")
	if block == nil {
		return
	}

	block.EntryValue("address", &cfg.PProfAddress)
}

func (cfg *ServiceCfg) initServers(doc *bcl.Document, protocolsInfo []*boulevard.ProtocolInfo) {
	protoNames := make([]string, len(protocolsInfo))
	for i, info := range protocolsInfo {
		protoNames[i] = info.Name
	}

	for _, block := range doc.FindBlocks("server") {
		block.CheckBlocksOneOf(protoNames...)

		var protoInfo *boulevard.ProtocolInfo
		var protoCfg boulevard.ProtocolCfg
		var proto boulevard.Protocol

		for _, info := range protocolsInfo {
			if block := block.FindBlock(info.Name); block != nil {
				protoInfo = info
				protoCfg = info.InstantiateCfg()
				proto = info.Instantiate()

				block.Extract(protoCfg)
				break
			}
		}

		serverCfg := boulevard.ServerCfg{
			Name:         block.BlockName(),
			Protocol:     proto,
			ProtocolInfo: protoInfo,
			ProtocolCfg:  protoCfg,
		}

		block.Extract(&serverCfg)
		cfg.Servers = append(cfg.Servers, &serverCfg)
	}
}

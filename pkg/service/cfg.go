package service

import (
	"errors"
	"fmt"
	"io"
	"os"

	"go.n16f.net/boulevard/pkg/boulevard"
	"go.n16f.net/ejson"
	"go.n16f.net/eyaml"
	"go.n16f.net/log"
)

type ServiceCfg struct {
	BuildId string `json:"-"` // [1]

	Logger       *log.LoggerCfg `json:"logger,omitempty"`
	ACME         *ACMECfg       `json:"acme,omitempty"`
	ControlAPI   *ControlAPICfg `json:"control_api,omitempty"`
	PProfAddress string         `json:"pprof_address,omitempty"`

	ModuleInfo []*boulevard.ModuleInfo `json:"-"` // [1]
	Modules    []*ModuleCfg            `json:"-"` // [2]

	// [1] Provided by the caller of NewService.
	// [2] Populated by ServiceCfg.Load.
}

func (cfg *ServiceCfg) ValidateJSON(v *ejson.Validator) {
	v.CheckOptionalObject("logger", cfg.Logger)

	if cfg.PProfAddress != "" {
		v.CheckNetworkAddress("pprof_address", cfg.PProfAddress)
	}

	v.CheckOptionalObject("acme", cfg.ACME)
	v.CheckOptionalObject("control_api", cfg.ControlAPI)
}

func (cfg *ServiceCfg) Load(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("cannot read %q: %w", filePath, err)
	}

	decoder := eyaml.NewDecoder(data)

	if err := decoder.Decode(cfg); err != nil {
		if errors.Is(err, io.EOF) {
			return fmt.Errorf("empty configuration")
		}

		return fmt.Errorf("cannot decode configuration: %w", err)
	}

	for {
		var modCfg ModuleCfg
		if err := decoder.Decode(&modCfg); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return fmt.Errorf("cannot decode module configuration: %w", err)
		}

		cfg.Modules = append(cfg.Modules, &modCfg)
	}

	return nil
}

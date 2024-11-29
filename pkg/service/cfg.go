package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"

	"go.n16f.net/boulevard/pkg/boulevard"
	"go.n16f.net/ejson"
	"go.n16f.net/eyaml"
	"go.n16f.net/log"
)

type ServiceCfg struct {
	BuildId string `json:"-"` // [1]

	Logger       *log.LoggerCfg `json:"logger,omitempty"`
	ACME         *ACMECfg       `json:"acme,omitempty"`
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
}

func (cfg *ServiceCfg) Load(filePath string) error {
	data, err := ioutil.ReadFile(filePath)
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

func (cfg *ServiceCfg) Dump(w io.Writer) error {
	// Ideally we would like the ability to dump the configuration using either
	// JSON or YAML (e.g. with a --dump-cfg-format command line option).
	// Infortunately gopkg.in/yaml.v3 does not support JSON tags and is a PITA
	// to use anyway. So we use the JSON encoder (valid since these are also
	// valid YAML documents) and add document boundaries ourselves. To be
	// revisited when go.n16f.net/yaml is ready.

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")

	if _, err := fmt.Fprintln(w, "---"); err != nil {
		return err
	}
	if err := encoder.Encode(cfg); err != nil {
		return err
	}

	for _, mod := range cfg.Modules {
		if _, err := fmt.Fprintln(w, "---"); err != nil {
			return err
		}

		if err := encoder.Encode(mod); err != nil {
			return err
		}
	}

	return nil
}

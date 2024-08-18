package service

import (
	"fmt"
	"io/ioutil"

	"go.n16f.net/ejson"
	"go.n16f.net/log"
	"go.n16f.net/yamlutils"
)

type ServiceCfg struct {
	BuildId string `json:"-"`

	Logger *log.LoggerCfg `json:"logger"`
}

func (cfg *ServiceCfg) ValidateJSON(v *ejson.Validator) {
	v.CheckOptionalObject("logger", cfg.Logger)
}

func (cfg *ServiceCfg) Load(filePath string) error {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("cannot read %q: %w", filePath, err)
	}

	return yamlutils.Load(data, cfg)
}

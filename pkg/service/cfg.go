package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"

	"go.n16f.net/ejson"
	"go.n16f.net/log"
	"gopkg.in/yaml.v3"
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

	// XXX All this mess will be replace by a single call to yaml.Load once
	// go.n16f.net/yaml is ready.

	yamlDecoder := yaml.NewDecoder(bytes.NewReader(data))

	var yamlValue any
	if err := yamlDecoder.Decode(&yamlValue); err != nil && err != io.EOF {
		return fmt.Errorf("cannot decode YAML data: %w", err)
	}

	jsonValue, err := YAMLValueToJSONValue(yamlValue)
	if err != nil {
		return fmt.Errorf("invalid YAML data: %w", err)
	}

	jsonData, err := json.Marshal(jsonValue)
	if err != nil {
		return fmt.Errorf("cannot generate JSON data: %w", err)
	}

	if err := ejson.Unmarshal(jsonData, cfg); err != nil {
		return fmt.Errorf("cannot decode JSON data: %w", err)
	}

	return nil
}

func YAMLValueToJSONValue(yamlValue any) (any, error) {
	// For some reason, go-yaml will return objects as map[string]any
	// if all keys are strings, and as map[any]any if not. So
	// we have to handle both.

	var jsonValue any

	switch v := yamlValue.(type) {
	case []any:
		array := make([]any, len(v))

		for i, yamlElement := range v {
			jsonElement, err := YAMLValueToJSONValue(yamlElement)
			if err != nil {
				return nil, err
			}

			array[i] = jsonElement
		}

		jsonValue = array

	case map[any]any:
		object := make(map[string]any)

		for key, yamlEntry := range v {
			keyString, ok := key.(string)
			if !ok {
				return nil,
					fmt.Errorf("object key \"%v\" is not a string", key)
			}

			jsonEntry, err := YAMLValueToJSONValue(yamlEntry)
			if err != nil {
				return nil, err
			}

			object[keyString] = jsonEntry
		}

		jsonValue = object

	case map[string]any:
		object := make(map[string]any)

		for key, yamlEntry := range v {
			jsonEntry, err := YAMLValueToJSONValue(yamlEntry)
			if err != nil {
				return nil, err
			}

			object[key] = jsonEntry
		}

		jsonValue = object

	default:
		jsonValue = yamlValue
	}

	return jsonValue, nil
}

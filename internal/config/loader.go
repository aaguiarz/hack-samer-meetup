package config

import (
	"io/ioutil"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"mapping-engine/internal/types"
)

// LoadMappingConfig loads mapping configuration from a YAML file
func LoadMappingConfig(configPath string) (*types.MappingConfig, error) {
	filename, err := filepath.Abs(configPath)
	if err != nil {
		return nil, err
	}

	yamlFile, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var config types.MappingConfig
	err = yaml.Unmarshal(yamlFile, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

// LoadMappingConfigs loads multiple mapping configurations from YAML files
func LoadMappingConfigs(configPaths []string) ([]*types.MappingConfig, error) {
	var configs []*types.MappingConfig

	for _, path := range configPaths {
		config, err := LoadMappingConfig(path)
		if err != nil {
			return nil, err
		}
		configs = append(configs, config)
	}

	return configs, nil
}

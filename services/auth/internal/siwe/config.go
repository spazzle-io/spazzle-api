package siwe

import (
	_ "embed"
	"fmt"

	"gopkg.in/yaml.v3"
)

//go:embed config/siwe-config.yaml
var defaultSIWEConfigYAML []byte

type Chain struct {
	Name         string   `yaml:"name"`
	ChainId      int32    `yaml:"chainId"`
	Environments []string `yaml:"environments"`
}

type Config struct {
	Chains []Chain `yaml:"chains"`
}

func (c *Config) getChain(chainId int32, environment string) *Chain {
	for _, chain := range c.Chains {
		if chain.ChainId == chainId && isEnvironmentValid(environment, chain.Environments) {
			return &chain
		}
	}

	return nil
}

func isEnvironmentValid(environment string, validEnvironments []string) bool {
	for _, validEnvironment := range validEnvironments {
		if validEnvironment == environment {
			return true
		}
	}

	return false
}

func loadDefaultSIWEConfig() (*Config, error) {
	var config Config
	if err := yaml.Unmarshal(defaultSIWEConfigYAML, &config); err != nil {
		return nil, fmt.Errorf("could not unmarshall SIWE config: %w", err)
	}

	return &config, nil
}

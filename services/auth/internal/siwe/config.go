package siwe

import (
	_ "embed"
	"fmt"

	"gopkg.in/yaml.v3"
)

//go:embed config/siwe-config.yaml
var defaultSIWEConfigYAML []byte

type Chain struct {
	Name                string   `yaml:"name"`
	ChainId             int32    `yaml:"chainId"`
	AllowedEnvironments []string `yaml:"allowedEnvironments"`
}

type Config struct {
	AllowedDomains []string `yaml:"allowedDomains"`
	AllowedChains  []Chain  `yaml:"allowedChains"`
}

func (c *Config) isDomainAllowed(domain string) bool {
	for _, allowedDomain := range c.AllowedDomains {
		if allowedDomain == domain {
			return true
		}
	}

	return false
}

func (c *Config) getChain(chainId int32, environment string) *Chain {
	for _, allowedChain := range c.AllowedChains {
		if allowedChain.ChainId == chainId && isEnvironmentValid(environment, allowedChain.AllowedEnvironments) {
			return &allowedChain
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

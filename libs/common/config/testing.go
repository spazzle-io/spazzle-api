package config

import "github.com/spf13/viper"

// NewTestChainRegistry constructs a ChainRegistry for use in tests.
// It bypasses the environment-based lookup and sets the chain directly.
//
// Only for use in tests. Do not call from production code.
func NewTestChainRegistry(chain Chain) *ChainRegistry {
	return &ChainRegistry{
		current: chain,
		byID:    map[uint64]Chain{chain.ID: chain},
	}
}

// NewTestAppConfig constructs an AppConfig with a preloaded viper instance.
// Use in tests that have no service Config struct.
func NewTestAppConfig(v *viper.Viper) *AppConfig {
	return &AppConfig{
		viper:       v,
		Environment: Development,
		ServiceName: v.GetString("SERVICE"),
	}
}

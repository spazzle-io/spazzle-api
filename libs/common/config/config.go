package config

import (
	"github.com/spf13/viper"
)

// LoadConfig loads configuration from a file and environment variables into a generic config struct.
//
// It searches for a configuration file with the specified name and path, expecting the file to be of type "env".
// It also automatically overrides config values with environment variables that match.
func LoadConfig[T any](path string, configName string) (config T, err error) {
	viper.AddConfigPath(path)
	viper.SetConfigName(configName)
	viper.SetConfigType("env")

	viper.AutomaticEnv()
	viper.SetTypeByDefaultValue(true)

	err = viper.ReadInConfig()
	if err != nil {
		return
	}

	err = viper.Unmarshal(&config)
	return
}

package util

type Config struct {
	Environment string `mapstructure:"ENVIRONMENT"`
	DBSource    string `mapstructure:"DB_SOURCE"`
}

package util

type Environment string

const Development Environment = "development"

type Config struct {
	Environment       Environment `mapstructure:"ENVIRONMENT"`
	ServiceName       string      `mapstructure:"SERVICE"`
	DBDriver          string      `mapstructure:"DB_DRIVER"`
	DBSource          string      `mapstructure:"DB_SOURCE"`
	AllowedOrigins    []string    `mapstructure:"ALLOWED_ORIGINS"`
	RedisConnURL      string      `mapstructure:"REDIS_CONN_URL"`
	DBMigrationURL    string      `mapstructure:"DB_MIGRATION_URL"`
	HTTPServerAddress string      `mapstructure:"HTTP_SERVER_ADDRESS"`
	GRPCServerAddress string      `mapstructure:"GRPC_SERVER_ADDRESS"`
}

func (c *Config) IsDevelopmentEnvironment() bool {
	return c.Environment == Development
}

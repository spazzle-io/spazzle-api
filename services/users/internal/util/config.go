package util

import commonConfig "github.com/spazzle-io/spazzle-api/libs/common/config"

type Config struct {
	commonConfig.AppConfig `mapstructure:",squash"`

	DBDriver                  string `mapstructure:"DB_DRIVER"`
	DBSource                  string `mapstructure:"DB_SOURCE"`
	RedisConnURL              string `mapstructure:"REDIS_CONN_URL"`
	DBMigrationURL            string `mapstructure:"DB_MIGRATION_URL"`
	HTTPServerAddress         string `mapstructure:"HTTP_SERVER_ADDRESS"`
	GRPCServerAddress         string `mapstructure:"GRPC_SERVER_ADDRESS"`
	AuthServiceGRPCServerAddr string `mapstructure:"AUTH_SERVICE_GRPC_SERVER_ADDRESS"`
}

func (c *Config) Base() *commonConfig.AppConfig {
	return &c.AppConfig
}

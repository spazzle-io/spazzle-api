package util

import (
	"time"

	commonConfig "github.com/spazzle-io/spazzle-api/libs/common/config"

	commonMiddleware "github.com/spazzle-io/spazzle-api/libs/common/middleware"
)

const Users commonMiddleware.Service = "users"

type Config struct {
	commonConfig.AppConfig `mapstructure:",squash"`

	DBDriver             string        `mapstructure:"DB_DRIVER"`
	DBSource             string        `mapstructure:"DB_SOURCE"`
	RedisConnURL         string        `mapstructure:"REDIS_CONN_URL"`
	DBMigrationURL       string        `mapstructure:"DB_MIGRATION_URL"`
	HTTPServerAddress    string        `mapstructure:"HTTP_SERVER_ADDRESS"`
	GRPCServerAddress    string        `mapstructure:"GRPC_SERVER_ADDRESS"`
	TokenSymmetricKey    string        `mapstructure:"TOKEN_SYMMETRIC_KEY"`
	AccessTokenDuration  time.Duration `mapstructure:"ACCESS_TOKEN_DURATION"`
	RefreshTokenDuration time.Duration `mapstructure:"REFRESH_TOKEN_DURATION"`
}

func (c *Config) Base() *commonConfig.AppConfig {
	return &c.AppConfig
}

package util

import (
	"time"

	commonMiddleware "github.com/spazzle-io/spazzle-api/libs/common/middleware"
)

type Environment string

const (
	Development Environment = "development"

	Users commonMiddleware.Service = "users"
)

type Config struct {
	Environment          Environment   `mapstructure:"ENVIRONMENT"`
	ServiceName          string        `mapstructure:"SERVICE"`
	DBDriver             string        `mapstructure:"DB_DRIVER"`
	DBSource             string        `mapstructure:"DB_SOURCE"`
	AllowedOrigins       []string      `mapstructure:"ALLOWED_ORIGINS"`
	RedisConnURL         string        `mapstructure:"REDIS_CONN_URL"`
	DBMigrationURL       string        `mapstructure:"DB_MIGRATION_URL"`
	HTTPServerAddress    string        `mapstructure:"HTTP_SERVER_ADDRESS"`
	GRPCServerAddress    string        `mapstructure:"GRPC_SERVER_ADDRESS"`
	TokenSymmetricKey    string        `mapstructure:"TOKEN_SYMMETRIC_KEY"`
	AccessTokenDuration  time.Duration `mapstructure:"ACCESS_TOKEN_DURATION"`
	RefreshTokenDuration time.Duration `mapstructure:"REFRESH_TOKEN_DURATION"`
}

func (c *Config) IsDevelopmentEnvironment() bool {
	return c.Environment == Development
}

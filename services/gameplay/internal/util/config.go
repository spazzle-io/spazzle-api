package util

type Environment string

const Development Environment = "development"

type Config struct {
	Environment               Environment `mapstructure:"ENVIRONMENT"`
	ServiceName               string      `mapstructure:"SERVICE"`
	DBDriver                  string      `mapstructure:"DB_DRIVER"`
	DBSource                  string      `mapstructure:"DB_SOURCE"`
	AllowedOrigins            []string    `mapstructure:"ALLOWED_ORIGINS"`
	RedisConnURL              string      `mapstructure:"REDIS_CONN_URL"`
	TemporalAPIKey            string      `mapstructure:"TEMPORAL_API_KEY"`
	TemporalHostPort          string      `mapstructure:"TEMPORAL_HOST_PORT"`
	ObjectStoreEndpoint       string      `mapstructure:"OBJECT_STORE_ENDPOINT"`
	ObjectStoreRegion         string      `mapstructure:"OBJECT_STORE_REGION"`
	ObjectStoreAccessKey      string      `mapstructure:"OBJECT_STORE_ACCESS_KEY"`
	ObjectStoreSecretKey      string      `mapstructure:"OBJECT_STORE_SECRET_KEY"`
	DBMigrationURL            string      `mapstructure:"DB_MIGRATION_URL"`
	HTTPServerAddress         string      `mapstructure:"HTTP_SERVER_ADDRESS"`
	GRPCServerAddress         string      `mapstructure:"GRPC_SERVER_ADDRESS"`
	AuthServiceGRPCServerAddr string      `mapstructure:"AUTH_SERVICE_GRPC_SERVER_ADDRESS"`
}

func (c *Config) IsDevelopmentEnvironment() bool {
	return c.Environment == Development
}

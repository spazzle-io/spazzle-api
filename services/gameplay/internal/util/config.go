package util

import commonConfig "github.com/spazzle-io/spazzle-api/libs/common/config"

type Config struct {
	commonConfig.AppConfig `mapstructure:",squash"`

	DBDriver                   string `mapstructure:"DB_DRIVER"`
	DBSource                   string `mapstructure:"DB_SOURCE"`
	RedisConnURL               string `mapstructure:"REDIS_CONN_URL"`
	TemporalAPIKey             string `mapstructure:"TEMPORAL_API_KEY"`
	TemporalHostPort           string `mapstructure:"TEMPORAL_HOST_PORT"`
	ObjectStoreEndpoint        string `mapstructure:"OBJECT_STORE_ENDPOINT"`
	ObjectStoreRegion          string `mapstructure:"OBJECT_STORE_REGION"`
	ObjectStoreAccessKey       string `mapstructure:"OBJECT_STORE_ACCESS_KEY"`
	ObjectStoreSecretKey       string `mapstructure:"OBJECT_STORE_SECRET_KEY"`
	DBMigrationURL             string `mapstructure:"DB_MIGRATION_URL"`
	HTTPServerAddress          string `mapstructure:"HTTP_SERVER_ADDRESS"`
	GRPCServerAddress          string `mapstructure:"GRPC_SERVER_ADDRESS"`
	AuthServiceGRPCServerAddr  string `mapstructure:"AUTH_SERVICE_GRPC_SERVER_ADDRESS"`
	TreasuryDeployerPrivateKey string `mapstructure:"TREASURY_DEPLOYER_PRIVATE_KEY"`
	RPCUrl                     string `mapstructure:"RPC_URL"`
}

func (c *Config) Base() *commonConfig.AppConfig {
	return &c.AppConfig
}

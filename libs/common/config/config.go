package config

import (
	"errors"
	"os"
	"reflect"

	"github.com/golang-migrate/migrate/v4"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
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

	if err = viper.ReadInConfig(); err != nil {
		var configFileNotFound viper.ConfigFileNotFoundError
		if !errors.As(err, &configFileNotFound) {
			return
		}
	}

	bindEnvs[T]()

	err = viper.Unmarshal(&config)
	return
}

func bindEnvs[T any]() {
	var t T

	val := reflect.TypeOf(t)
	if val.Kind() == reflect.Pointer {
		val = val.Elem()
	}

	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		tag := field.Tag.Get("mapstructure")
		if tag != "" {
			err := viper.BindEnv(tag)
			if err != nil {
				log.Warn().Err(err).Str("tag", tag).Msg("could not bind env var")
			}
		}
	}
}

// SetupLogger configures the default service logger
func SetupLogger(serviceName string, isDevelopmentEnvironment bool) {
	logger := log.Logger

	if isDevelopmentEnvironment {
		logger = logger.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}

	logger = logger.With().Str("service", serviceName).Logger()
	log.Logger = logger
}

// RunDBMigration runs migrate up on the specified dbSource using the migrationURL
func RunDBMigration(migrationURL string, dbSource string) {
	migration, err := migrate.New(migrationURL, dbSource)
	if err != nil {
		log.Fatal().Err(err).Msg("could not create new migration instance")
	}

	err = migration.Up()
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		log.Fatal().Err(err).Msg("failed to run migration")
	}

	log.Info().Msg("db migrated successfully")
}

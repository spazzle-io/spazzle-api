package config

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/mitchellh/mapstructure"

	"github.com/golang-migrate/migrate/v4"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

// AppConfig is the required base for every service configuration struct.
// It holds the fields that every service must define.
//
// Services embed AppConfig and implement getBase() to participate in Load:
//
//	type Config struct {
//	    AppConfig `mapstructure:",squash"`
//
//	    DBSource          string        `mapstructure:"DB_SOURCE"`
//	    HTTPServerAddress string        `mapstructure:"HTTP_SERVER_ADDRESS"`
//	    // ... service-specific fields
//	}
//
//	func (c *Config) Base() *commonConfig.AppConfig {
//	    return &c.AppConfig
//	}
type AppConfig struct {
	viper *viper.Viper `mapstructure:"-"`

	Environment    Environment    `mapstructure:"ENVIRONMENT"`
	ServiceName    string         `mapstructure:"SERVICE"`
	AllowedOrigins []string       `mapstructure:"ALLOWED_ORIGINS"`
	Chains         *ChainRegistry `mapstructure:"-"`
}

func (a *AppConfig) Is(env Environment) bool {
	return a.Environment.Is(env)
}

// GetStringSlice retrieves a dynamic config key as a string slice.
// Use for keys that are not known at compile time.
func (a *AppConfig) GetStringSlice(key string) []string {
	raw := a.viper.GetString(key)
	if raw == "" {
		return []string{}
	}

	return strings.Split(raw, ",")
}

type baseProvider interface {
	Base() *AppConfig
}

// Load loads and fully initialises a service configuration in three steps:
//
//  1. Calls LoadConfig[T] to unmarshal the env file and environment variables.
//  2. Validates the provided environment.
//  3. Builds the ChainRegistry for the resolved environment and attaches it to AppConfig.Chains.
//
// T must embed AppConfig and implement getBase()
func Load[T baseProvider](path string, name string) (T, error) {
	cfg, v, err := loadConfigInternal[T](path, name)
	if err != nil {
		return cfg, fmt.Errorf("failed to load config: %w", err)
	}

	base := cfg.Base()

	if err := base.Environment.IsValid(); err != nil {
		return cfg, err
	}

	chains, err := newChainRegistry(base.Environment)
	if err != nil {
		return cfg, err
	}

	base.ServiceName = strings.ToLower(strings.TrimSpace(base.ServiceName))
	base.Chains = chains
	base.viper = v

	return cfg, nil
}

// LoadConfig loads a configuration file and environment variables into T.
//
// It searches for a file at <path>/<configName>.env and unmarshals it into T using mapstructure tags.
// Environment variables matching any tag are automatically applied on top, allowing the file to act as
// a baseline that deployments override at runtime.
//
// Fields tagged with mapstructure:"-" are skipped.
//
// This is the low-level loader. Most services should call Load instead, which runs LoadConfig and
// additionally validates the environment and wires the chain registry.
func LoadConfig[T any](path string, configName string) (config T, err error) {
	config, _, err = loadConfigInternal[T](path, configName)
	return config, err
}

func loadConfigInternal[T any](path string, configName string) (config T, v *viper.Viper, err error) {
	v = viper.New()
	v.AddConfigPath(path)
	v.SetConfigName(configName)
	v.SetConfigType("env")

	v.AutomaticEnv()

	if readErr := v.ReadInConfig(); readErr != nil {
		var configFileNotFound viper.ConfigFileNotFoundError
		if !errors.As(readErr, &configFileNotFound) {
			err = readErr
			return
		}
	}

	bindEnvsV[T](v)
	err = v.Unmarshal(&config, viper.DecodeHook(
		mapstructure.ComposeDecodeHookFunc(
			mapstructure.StringToTimeDurationHookFunc(),
			mapstructure.StringToSliceHookFunc(","),
		),
	))

	return
}

func bindEnvsV[T any](v *viper.Viper) {
	var t T

	val := reflect.TypeOf(t)
	for val != nil && val.Kind() == reflect.Pointer {
		val = val.Elem()
	}
	if val == nil || val.Kind() != reflect.Struct {
		return
	}

	bindStructFields(v, val)
}

func bindStructFields(v *viper.Viper, t reflect.Type) {
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.Anonymous && field.Type.Kind() == reflect.Struct {
			bindStructFields(v, field.Type)
			continue
		}

		tag := field.Tag.Get("mapstructure")
		if tag == "" || tag == "-" {
			continue
		}

		if err := v.BindEnv(tag); err != nil {
			log.Warn().Err(err).Str("tag", tag).Msg("could not bind env var")
		}
	}
}

// SetupLogger configures the default service logger
func SetupLogger(serviceName string, isDevelopmentEnvironment bool) {
	if isDevelopmentEnvironment {
		log.Logger = log.Logger.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}

	log.Logger = log.Logger.With().Str("service", serviceName).Logger()
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

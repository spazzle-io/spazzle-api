package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

type TestConfig struct {
	DBDriver string `mapstructure:"DB_DRIVER"`
}

func TestLoadConfig(t *testing.T) {
	config, err := LoadConfig[TestConfig]("./testdata", "app")
	require.NoError(t, err)

	require.Equal(t, "test_db_driver", config.DBDriver)
}

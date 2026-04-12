package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnvironment_IsValid(t *testing.T) {
	t.Run("valid environment", func(t *testing.T) {
		environment := Staging
		require.NoError(t, environment.IsValid())
	})

	t.Run("invalid environment", func(t *testing.T) {
		environment := Environment("invalid")
		require.Error(t, environment.IsValid())
	})
}

func TestEnvironment_Is(t *testing.T) {
	environment := Production
	require.True(t, environment.Is(Production))
	require.False(t, environment.Is(Staging))
}

func TestEnvironment_String(t *testing.T) {
	environment := Development
	require.Equal(t, "development", environment.String())
}

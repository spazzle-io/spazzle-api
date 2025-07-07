package util

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestConfig_IsDevelopmentEnvironment(t *testing.T) {
	testCases := []struct {
		name        string
		environment Environment
		expected    bool
	}{
		{
			name:        "is development environment",
			environment: Development,
			expected:    true,
		},
		{
			name:        "is not development environment",
			environment: "production",
			expected:    false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := Config{
				Environment: tc.environment,
			}
			require.Equal(t, tc.expected, config.IsDevelopmentEnvironment())
		})
	}
}

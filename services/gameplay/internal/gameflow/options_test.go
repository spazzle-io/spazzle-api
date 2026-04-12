package gameflow

import (
	"testing"

	commonConfig "github.com/spazzle-io/spazzle-api/libs/common/config"

	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/util"
	"github.com/stretchr/testify/require"
)

func TestGetTemporalNamespace(t *testing.T) {
	testCases := []struct {
		name              string
		config            util.Config
		expectedNamespace string
	}{
		{
			name: "dev namespace",
			config: util.Config{
				AppConfig: commonConfig.AppConfig{
					ServiceName: "test",
					Environment: "dev",
				},
			},
			expectedNamespace: "test-dev",
		},
		{
			name: "production namespace",
			config: util.Config{
				AppConfig: commonConfig.AppConfig{
					ServiceName: "gameplay",
					Environment: "production",
				},
			},
			expectedNamespace: "gameplay-production",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gotNamespace := getTemporalNamespace(&tc.config)
			require.Equal(t, tc.expectedNamespace, gotNamespace)
		})
	}
}

func TestGetTemporalClientOpts(t *testing.T) {
	testCases := []struct {
		name           string
		config         util.Config
		hasCredentials bool
	}{
		{
			name: "dev environment",
			config: util.Config{
				AppConfig: commonConfig.AppConfig{
					Environment: commonConfig.Development,
					ServiceName: "gameplay",
				},
				TemporalHostPort: "some-host-port",
				TemporalAPIKey:   "some-api-key",
			},
			hasCredentials: false,
		},
		{
			name: "production environment",
			config: util.Config{
				AppConfig: commonConfig.AppConfig{
					Environment: "production",
					ServiceName: "gameplay",
				},
				TemporalHostPort: "some-host-port",
				TemporalAPIKey:   "some-api-key",
			},
			hasCredentials: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gotOpts := getTemporalClientOpts(&tc.config)
			if tc.hasCredentials {
				require.NotEmpty(t, gotOpts.HostPort)
				require.NotEmpty(t, gotOpts.Credentials)
				return
			}

			require.Empty(t, gotOpts.HostPort)
			require.Empty(t, gotOpts.Credentials)
		})
	}
}

package workflow

import (
	"testing"

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
				ServiceName: "test",
				Environment: "dev",
			},
			expectedNamespace: "test-dev",
		},
		{
			name: "production namespace",
			config: util.Config{
				ServiceName: "gameplay",
				Environment: "production",
			},
			expectedNamespace: "gameplay-production",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gotNamespace := getTemporalNamespace(tc.config)
			require.Equal(t, tc.expectedNamespace, gotNamespace)
		})
	}
}

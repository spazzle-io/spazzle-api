package websocketserver

import (
	"net/http"
	"testing"

	commonConfig "github.com/spazzle-io/spazzle-api/libs/common/config"

	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/api/deps"

	"github.com/google/uuid"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/util"
	"github.com/stretchr/testify/require"
)

func TestCheckOrigin(t *testing.T) {
	userId := uuid.New()

	testCases := []struct {
		name          string
		originHeader  string
		allowed       []string
		isDevEnv      bool
		expectedValid bool
	}{
		{
			name:          "success",
			originHeader:  "https://spazzle.io",
			allowed:       []string{"https://spazzle.io"},
			isDevEnv:      false,
			expectedValid: true,
		},
		{
			name:          "no origin header",
			originHeader:  "",
			allowed:       []string{},
			isDevEnv:      false,
			expectedValid: true,
		},
		{
			name:          "development environment ignores origin",
			originHeader:  "https://somesite.com",
			allowed:       []string{},
			isDevEnv:      true,
			expectedValid: true,
		},
		{
			name:          "invalid origin URL",
			originHeader:  "https://someinvalid.com",
			allowed:       []string{"https://example.com"},
			isDevEnv:      false,
			expectedValid: false,
		},
		{
			name:          "allowed origin exact match",
			originHeader:  "https://spazzle.io",
			allowed:       []string{"https://google.com", "https://spazzle.io"},
			isDevEnv:      false,
			expectedValid: true,
		},
		{
			name:          "allowed origin subdomain match",
			originHeader:  "https://api.spazzle.io",
			allowed:       []string{"https://spazzle.io"},
			isDevEnv:      false,
			expectedValid: true,
		},
		{
			name:          "disallowed origin different domain",
			originHeader:  "https://evil.com",
			allowed:       []string{"https://spazzle.io"},
			isDevEnv:      false,
			expectedValid: false,
		},
		{
			name:          "origin has suffix of an allowed domain",
			originHeader:  "https://evilspazzle.io",
			allowed:       []string{"https://spazzle.io"},
			isDevEnv:      false,
			expectedValid: false,
		},
		{
			name:          "allowed origin with www subdomain",
			originHeader:  "https://www.sub.spazzle.io",
			allowed:       []string{"https://spazzle.io"},
			isDevEnv:      false,
			expectedValid: true,
		},
		{
			name:          "case insensitive matching",
			originHeader:  "Https://spAZzle.iO",
			allowed:       []string{"htTPs://sPazzLe.io"},
			isDevEnv:      false,
			expectedValid: true,
		},
		{
			name:          "scheme mismatch",
			originHeader:  "http://spazzle.io",
			allowed:       []string{"https://spazzle.io"},
			isDevEnv:      false,
			expectedValid: false,
		},
		{
			name:          "allowed origin parse error ignored gracefully",
			originHeader:  "https://spazzle.io",
			allowed:       []string{"https://%%%/"}, // invalid URL in config
			isDevEnv:      false,
			expectedValid: false,
		},
		{
			name:          "invalid origin URL",
			originHeader:  "https://%%%/",
			allowed:       []string{"https://spazzle.io"},
			isDevEnv:      false,
			expectedValid: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			environment := "development"
			if !tc.isDevEnv {
				environment = "production"
			}

			cfg := &util.Config{
				AppConfig: commonConfig.AppConfig{
					AllowedOrigins: tc.allowed,
					Environment:    commonConfig.Environment(environment),
				},
			}

			req := &http.Request{
				Header: make(http.Header),
			}
			if tc.originHeader != "" {
				req.Header.Set("Origin", tc.originHeader)
			}

			wsHandler := &WsHandler{
				APIServerDeps: &deps.APIServerDeps{
					Config: cfg,
				},
			}
			checkFn := wsHandler.checkOrigin(userId)
			valid := checkFn(req)

			require.Equal(t, tc.expectedValid, valid)
		})
	}
}

package websocketserver

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	commonMiddleware "github.com/spazzle-io/spazzle-api/libs/common/middleware"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/util"
	"github.com/stretchr/testify/require"
)

func TestServeWS(t *testing.T) {
	validUserID := uuid.New()
	validServerID := uuid.New()

	sm := createTestGameServerManager(t)
	require.NotEmpty(t, sm)

	testCases := []struct {
		name      string
		query     url.Values
		config    util.Config
		shouldErr bool
	}{
		{
			name:  "success",
			query: url.Values{"user_id": {validUserID.String()}, "server_id": {validServerID.String()}},
			config: util.Config{
				Environment:    "production",
				AllowedOrigins: []string{"https://spazzle.io"},
			},
			shouldErr: false,
		},
		{
			name:  "error extracting request metadata",
			query: url.Values{},
			config: util.Config{
				Environment:    "production",
				AllowedOrigins: []string{"https://spazzle.io"},
			},
			shouldErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			initialStartClientPumps := startClientPumps
			startClientPumps = func(ctx context.Context, c *Client) {}
			defer func() {
				startClientPumps = initialStartClientPumps
			}()

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, err := ServeWs(context.Background(), sm, tc.config, w, r)
				if tc.shouldErr {
					require.Error(t, err)
					return
				}

				require.NoError(t, err)
			}))

			urlStr := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws?" + tc.query.Encode()

			validToken := "some_valid_token"
			authHeader := fmt.Sprintf("%s %s", commonMiddleware.AuthorizationBearer, validToken)

			header := http.Header{}
			header.Add(commonMiddleware.AuthorizationHeader, authHeader)

			_, _, err := websocket.DefaultDialer.Dial(urlStr, header)
			if !tc.shouldErr {
				require.NoError(t, err)
			}
		})
	}
}

func TestExtractRequestMetadata(t *testing.T) {
	validUserID := uuid.New()
	validServerID := uuid.New()
	validToken := "some_token"

	testCases := []struct {
		name             string
		query            url.Values
		authHeader       string
		expectedUserID   uuid.UUID
		expectedServerID uuid.UUID
		expectedJoinCode string
		expectedStatus   int
		expectedErr      error
	}{
		{
			name:             "success",
			query:            url.Values{"user_id": {validUserID.String()}, "server_id": {validServerID.String()}},
			authHeader:       fmt.Sprintf("%s %s", commonMiddleware.AuthorizationBearer, validToken),
			expectedStatus:   http.StatusOK,
			expectedErr:      nil,
			expectedUserID:   validUserID,
			expectedServerID: validServerID,
			expectedJoinCode: validToken,
		},
		{
			name:           "missing user id",
			query:          url.Values{"server_id": {validServerID.String()}},
			authHeader:     fmt.Sprintf("%s %s", commonMiddleware.AuthorizationBearer, validToken),
			expectedStatus: http.StatusBadRequest,
			expectedErr:    ErrMissingUserID,
		},
		{
			name:           "missing server id",
			query:          url.Values{"user_id": {validUserID.String()}},
			authHeader:     fmt.Sprintf("%s %s", commonMiddleware.AuthorizationBearer, validToken),
			expectedStatus: http.StatusBadRequest,
			expectedErr:    ErrMissingServerID,
		},
		{
			name:           "invalid server id",
			query:          url.Values{"user_id": {validUserID.String()}, "server_id": {"fake-server-id"}},
			authHeader:     fmt.Sprintf("%s %s", commonMiddleware.AuthorizationBearer, validToken),
			expectedStatus: http.StatusInternalServerError,
			expectedErr:    ErrInternalServerError,
		},
		{
			name:           "invalid user id",
			query:          url.Values{"server_id": {validServerID.String()}, "user_id": {"fake-user-id"}},
			authHeader:     fmt.Sprintf("%s %s", commonMiddleware.AuthorizationBearer, validToken),
			expectedStatus: http.StatusInternalServerError,
			expectedErr:    ErrInternalServerError,
		},
		{
			name:           "missing authorization header",
			query:          url.Values{"user_id": {validUserID.String()}, "server_id": {validServerID.String()}},
			authHeader:     "",
			expectedStatus: http.StatusUnauthorized,
			expectedErr:    ErrMissingAuthorizationHeader,
		},
		{
			name:           "invalid authorization header format",
			query:          url.Values{"user_id": {validUserID.String()}, "server_id": {validServerID.String()}},
			authHeader:     fmt.Sprintf("invalid %s", validToken),
			expectedStatus: http.StatusUnauthorized,
			expectedErr:    ErrInvalidAuthorizationHeader,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/ws?"+tc.query.Encode(), nil)
			if tc.authHeader != "" {
				req.Header.Set(commonMiddleware.AuthorizationHeader, tc.authHeader)
			}
			rec := httptest.NewRecorder()

			userID, serverID, joinCode, err := extractRequestMetadata(rec, req)

			if tc.expectedErr != nil {
				require.Error(t, err)
				require.Equal(t, tc.expectedErr, err)
				require.Equal(t, tc.expectedStatus, rec.Code)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.expectedUserID, userID)
			require.Equal(t, tc.expectedServerID, serverID)
			require.Equal(t, tc.expectedJoinCode, joinCode)
			require.Equal(t, http.StatusOK, rec.Code)
		})
	}
}

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
			name:          "scheme mismatch",
			originHeader:  "http://spazzle.io",
			allowed:       []string{"https://spazzle.io"},
			isDevEnv:      false,
			expectedValid: false,
		},
		{
			name:          "allowed origin ignores scheme when allowed has none",
			originHeader:  "https://spazzle.io",
			allowed:       []string{"spazzle.io"},
			isDevEnv:      false,
			expectedValid: true,
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

			cfg := util.Config{
				AllowedOrigins: tc.allowed,
				Environment:    util.Environment(environment),
			}

			req := &http.Request{
				Header: make(http.Header),
			}
			if tc.originHeader != "" {
				req.Header.Set("Origin", tc.originHeader)
			}

			checkFn := checkOrigin(userId, cfg)
			valid := checkFn(req)

			require.Equal(t, tc.expectedValid, valid)
		})
	}
}

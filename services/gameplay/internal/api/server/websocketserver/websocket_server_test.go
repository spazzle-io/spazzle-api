package websocketserver

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	mockworkflowclient "github.com/spazzle-io/spazzle-api/services/gameplay/internal/workflow/mock"

	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameserver"

	mockcache "github.com/spazzle-io/spazzle-api/libs/common/cache/mock"
	"go.uber.org/mock/gomock"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	commonMiddleware "github.com/spazzle-io/spazzle-api/libs/common/middleware"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/util"
	"github.com/stretchr/testify/require"
)

func TestServeWS(t *testing.T) {
	validUserID := uuid.New()
	validServerID := uuid.New()
	validServerJoinCode := "some_valid_token"

	testCases := []struct {
		name       string
		query      url.Values
		config     util.Config
		buildStubs func(cache *mockcache.MockCache)
		shouldErr  bool
	}{
		{
			name:  "success",
			query: url.Values{"user_id": {validUserID.String()}, "server_id": {validServerID.String()}},
			config: util.Config{
				Environment:    "production",
				AllowedOrigins: []string{"https://spazzle.io"},
			},
			buildStubs: func(cache *mockcache.MockCache) {
				cache.EXPECT().
					Get(gomock.Any(), gomock.Any()).
					Times(1).
					Return(validServerJoinCode, nil)

				cache.EXPECT().
					Del(gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)
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
			buildStubs: func(cache *mockcache.MockCache) {},
			shouldErr:  true,
		},
		{
			name:  "error fetching server join code",
			query: url.Values{"user_id": {validUserID.String()}, "server_id": {validServerID.String()}},
			config: util.Config{
				Environment:    "production",
				AllowedOrigins: []string{"https://spazzle.io"},
			},
			buildStubs: func(cache *mockcache.MockCache) {
				cache.EXPECT().
					Get(gomock.Any(), gomock.Any()).
					Times(1).
					Return("", errors.New("failed to fetch server join code"))
			},
			shouldErr: true,
		},
		{
			name:  "server join code mismatch",
			query: url.Values{"user_id": {validUserID.String()}, "server_id": {validServerID.String()}},
			config: util.Config{
				Environment:    "production",
				AllowedOrigins: []string{"https://spazzle.io"},
			},
			buildStubs: func(cache *mockcache.MockCache) {
				cache.EXPECT().
					Get(gomock.Any(), gomock.Any()).
					Times(1).
					Return("invalid-server-join-code", nil)

				cache.EXPECT().
					Del(gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)
			},
			shouldErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var wg sync.WaitGroup
			wg.Add(1)

			crtl := gomock.NewController(t)
			defer crtl.Finish()

			cache := mockcache.NewMockCache(crtl)
			wfClient := mockworkflowclient.NewMockClient(crtl)

			sm := gameserver.NewManager(wfClient)
			require.NotEmpty(t, sm)

			tc.buildStubs(cache)

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer wg.Done()
				_, err := ServeWs(context.Background(), sm, tc.config, cache, w, r, &ServeWsOptions{StartPumps: false})
				if tc.shouldErr {
					require.Error(t, err)
					return
				}

				require.NoError(t, err)
			}))

			urlStr := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws?" + tc.query.Encode()

			authHeader := fmt.Sprintf("%s %s", commonMiddleware.AuthorizationBearer, validServerJoinCode)

			header := http.Header{}
			header.Add(commonMiddleware.AuthorizationHeader, authHeader)

			_, _, err := websocket.DefaultDialer.Dial(urlStr, header)
			if !tc.shouldErr {
				require.NoError(t, err)
			}

			wg.Wait()
			server.Close()
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

func TestGetServerJoinCodeCacheKey(t *testing.T) {
	userId := uuid.New()
	serverID := uuid.New()
	config := util.Config{
		ServiceName: "gameplay",
	}

	expectedCacheKey := fmt.Sprintf("gameplay-%s:%s:%s", serverJoinCodeCachePrefix, serverID.String(), userId.String())
	cacheKey := getServerJoinCodeCacheKey(userId, serverID, config)

	require.Equal(t, expectedCacheKey, cacheKey)
}

func TestGenerateServerJoinCode(t *testing.T) {
	testCases := []struct {
		name       string
		buildStubs func(cache *mockcache.MockCache)
		shouldErr  bool
	}{
		{
			name: "success",
			buildStubs: func(cache *mockcache.MockCache) {
				cache.EXPECT().
					Set(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)
			},
			shouldErr: false,
		},
		{
			name: "could not cache server join code",
			buildStubs: func(cache *mockcache.MockCache) {
				cache.EXPECT().
					Set(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(errors.New("could not cache server join code"))
			},
			shouldErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			crtl := gomock.NewController(t)
			defer crtl.Finish()

			config := util.Config{
				ServiceName: "test",
			}

			cache := mockcache.NewMockCache(crtl)
			tc.buildStubs(cache)

			joinCode, err := GenerateServerJoinCode(context.Background(), uuid.New(), uuid.New(), config, cache)
			if tc.shouldErr {
				require.Error(t, err)
				require.Empty(t, joinCode)
				return
			}

			require.NoError(t, err)
			require.NotEmpty(t, joinCode)
			require.Len(t, joinCode, nonceLength)
		})
	}
}

func TestFetchServerJoinCode(t *testing.T) {
	testCases := []struct {
		name             string
		buildStubs       func(cache *mockcache.MockCache)
		expectedJoinCode string
		expectedErr      error
	}{
		{
			name: "success",
			buildStubs: func(cache *mockcache.MockCache) {
				cache.EXPECT().
					Get(gomock.Any(), gomock.Any()).
					Times(1).
					Return("valid-server-join-code", nil)

				cache.EXPECT().
					Del(gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)
			},
			expectedErr:      nil,
			expectedJoinCode: "valid-server-join-code",
		},
		{
			name: "error retrieving server join code from cache",
			buildStubs: func(cache *mockcache.MockCache) {
				cache.EXPECT().
					Get(gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil, errors.New("error retrieving server join code"))
			},
			expectedErr:      ErrInternalServerError,
			expectedJoinCode: "",
		},
		{
			name: "server join code not found in cache",
			buildStubs: func(cache *mockcache.MockCache) {
				cache.EXPECT().
					Get(gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil, nil)
			},
			expectedErr:      ErrUnauthorizedAccess,
			expectedJoinCode: "",
		},
		{
			name: "could not cast server join code to string",
			buildStubs: func(cache *mockcache.MockCache) {
				cache.EXPECT().
					Get(gomock.Any(), gomock.Any()).
					Times(1).
					Return(util.Config{}, nil)
			},
			expectedErr:      ErrInternalServerError,
			expectedJoinCode: "",
		},
		{
			name: "could not delete server join code from cache",
			buildStubs: func(cache *mockcache.MockCache) {
				cache.EXPECT().
					Get(gomock.Any(), gomock.Any()).
					Times(1).
					Return("valid-server-join-code", nil)

				cache.EXPECT().
					Del(gomock.Any(), gomock.Any()).
					Times(1).
					Return(errors.New("could not delete server join code"))
			},
			expectedErr:      ErrInternalServerError,
			expectedJoinCode: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			crtl := gomock.NewController(t)
			defer crtl.Finish()

			config := util.Config{
				ServiceName: "test",
			}

			cache := mockcache.NewMockCache(crtl)
			tc.buildStubs(cache)

			recorder := httptest.NewRecorder()

			joinCode, err := fetchServerJoinCode(context.Background(), recorder, uuid.New(), uuid.New(), config, cache)
			if tc.expectedErr != nil {
				require.Error(t, err)
				require.ErrorIs(t, err, tc.expectedErr)
				require.Empty(t, joinCode)
				return
			}

			require.NoError(t, err)
			require.NotEmpty(t, joinCode)
		})
	}
}

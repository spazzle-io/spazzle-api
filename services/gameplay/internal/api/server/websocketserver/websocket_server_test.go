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

	db "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/sqlc"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/eventbus"

	commonCache "github.com/spazzle-io/spazzle-api/libs/common/cache"
	mockdb "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/mock"
	mockwordstore "github.com/spazzle-io/spazzle-api/services/gameplay/internal/wordstore/mock"

	mockeventbus "github.com/spazzle-io/spazzle-api/services/gameplay/internal/eventbus/mock"
	mockgameflowclient "github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow/mock"

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
		buildStubs func(store *mockdb.MockStore, bus *mockeventbus.MockEventBus, session *mockeventbus.MockSession, gfClient *mockgameflowclient.MockClient, cache *mockcache.MockCache)
		shouldErr  bool
	}{
		{
			name:  "success",
			query: url.Values{"user_id": {validUserID.String()}, "server_id": {validServerID.String()}},
			config: util.Config{
				Environment:    "production",
				AllowedOrigins: []string{"https://spazzle.io"},
			},
			buildStubs: func(store *mockdb.MockStore, bus *mockeventbus.MockEventBus, session *mockeventbus.MockSession, gfClient *mockgameflowclient.MockClient, cache *mockcache.MockCache) {
				cache.EXPECT().
					Get(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(ctx context.Context, key string, dest *string) error {
						*dest = validServerJoinCode
						return nil
					})

				cache.EXPECT().
					Del(gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)

				store.EXPECT().
					GetServerById(gomock.Any(), gomock.Eq(validServerID)).
					AnyTimes().
					Return(db.Server{
						NumDrawingOptions: 3,
					}, nil)

				gfClient.EXPECT().
					Game(gomock.Any(), gomock.Any()).
					AnyTimes().
					Return(uuid.New(), nil)

				gfClient.EXPECT().
					HeartbeatGameServerInstance(gomock.Any(), gomock.Any(), gomock.Any()).
					AnyTimes().
					Return(nil)

				gfClient.EXPECT().
					GetGameState(gomock.Any()).
					AnyTimes().
					Return(nil, nil)

				bus.EXPECT().
					Session(gomock.Any()).
					AnyTimes().
					Return(session, nil)

				session.EXPECT().
					Subscribe(gomock.Any(), gomock.Eq(eventbus.GameEventsStreamType), gomock.Any(), gomock.Any()).
					AnyTimes().
					Return(nil)

				session.EXPECT().
					Subscribe(gomock.Any(), gomock.Eq(eventbus.DrawingUpdatesStreamType), gomock.Any(), gomock.Any()).
					AnyTimes().
					Return(nil)

				gfClient.EXPECT().
					AddPlayers(gomock.Any(), gomock.Any(), gomock.Any()).
					AnyTimes().
					Return()
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
			buildStubs: func(store *mockdb.MockStore, bus *mockeventbus.MockEventBus, session *mockeventbus.MockSession, gfClient *mockgameflowclient.MockClient, cache *mockcache.MockCache) {
			},
			shouldErr: true,
		},
		{
			name:  "error fetching server join code",
			query: url.Values{"user_id": {validUserID.String()}, "server_id": {validServerID.String()}},
			config: util.Config{
				Environment:    "production",
				AllowedOrigins: []string{"https://spazzle.io"},
			},
			buildStubs: func(store *mockdb.MockStore, bus *mockeventbus.MockEventBus, session *mockeventbus.MockSession, gfClient *mockgameflowclient.MockClient, cache *mockcache.MockCache) {
				cache.EXPECT().
					Get(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(ctx context.Context, key string, dest *string) error {
						*dest = ""
						return errors.New("failed to fetch server join code")
					})
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
			buildStubs: func(store *mockdb.MockStore, bus *mockeventbus.MockEventBus, session *mockeventbus.MockSession, gfClient *mockgameflowclient.MockClient, cache *mockcache.MockCache) {
				cache.EXPECT().
					Get(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(ctx context.Context, key string, dest *string) error {
						*dest = "invalid-server-join-code"
						return nil
					})

				cache.EXPECT().
					Del(gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)
			},
			shouldErr: true,
		},
		{
			name:  "server join code case sensitivity",
			query: url.Values{"user_id": {validUserID.String()}, "server_id": {validServerID.String()}},
			config: util.Config{
				Environment:    "production",
				AllowedOrigins: []string{"https://spazzle.io"},
			},
			buildStubs: func(store *mockdb.MockStore, bus *mockeventbus.MockEventBus, session *mockeventbus.MockSession, gfClient *mockgameflowclient.MockClient, cache *mockcache.MockCache) {
				cache.EXPECT().
					Get(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(ctx context.Context, key string, dest *string) error {
						*dest = strings.ToUpper(validServerJoinCode)
						return nil
					})

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

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			cache := mockcache.NewMockCache(ctrl)
			bus := mockeventbus.NewMockEventBus(ctrl)
			session := mockeventbus.NewMockSession(ctrl)
			gfClient := mockgameflowclient.NewMockClient(ctrl)
			wordStore := mockwordstore.NewMockStore(ctrl)

			sm := gameserver.NewManager()
			require.NotEmpty(t, sm)

			wsHandler := &WsHandler{
				Config:    tc.config,
				Store:     store,
				Cache:     cache,
				GfClient:  gfClient,
				Bus:       bus,
				GsManager: sm,
				WordStore: wordStore,
			}

			tc.buildStubs(store, bus, session, gfClient, cache)

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer wg.Done()
				_, err := wsHandler.ServeWs(context.Background(), w, r, WithoutPumps())
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
		buildStubs       func(store *mockdb.MockStore)
		query            url.Values
		authHeader       string
		expectedRole     Role
		expectedUserID   uuid.UUID
		expectedServerID uuid.UUID
		expectedJoinCode string
		expectedStatus   int
		expectedErr      error
	}{
		{
			name:             "success",
			buildStubs:       func(store *mockdb.MockStore) {},
			query:            url.Values{"user_id": {validUserID.String()}, "server_id": {validServerID.String()}, "role": {"player"}},
			authHeader:       fmt.Sprintf("%s %s", commonMiddleware.AuthorizationBearer, validToken),
			expectedStatus:   http.StatusOK,
			expectedErr:      nil,
			expectedRole:     Player,
			expectedUserID:   validUserID,
			expectedServerID: validServerID,
			expectedJoinCode: validToken,
		},
		{
			name:             "success - role not provided",
			buildStubs:       func(store *mockdb.MockStore) {},
			query:            url.Values{"user_id": {validUserID.String()}, "server_id": {validServerID.String()}},
			authHeader:       fmt.Sprintf("%s %s", commonMiddleware.AuthorizationBearer, validToken),
			expectedStatus:   http.StatusOK,
			expectedErr:      nil,
			expectedRole:     Player,
			expectedUserID:   validUserID,
			expectedServerID: validServerID,
			expectedJoinCode: validToken,
		},
		{
			name: "success - moderator",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetServerUserPermissions(gomock.Any(), db.GetServerUserPermissionsParams{
						ServerID: validServerID,
						UserID:   validUserID,
					}).
					Times(1).
					Return(db.GetServerUserPermissionsRow{
						HasElevatedPermissions: true,
					}, nil)
			},
			query:            url.Values{"user_id": {validUserID.String()}, "server_id": {validServerID.String()}, "role": {"moderator"}},
			authHeader:       fmt.Sprintf("%s %s", commonMiddleware.AuthorizationBearer, validToken),
			expectedStatus:   http.StatusOK,
			expectedErr:      nil,
			expectedRole:     Moderator,
			expectedUserID:   validUserID,
			expectedServerID: validServerID,
			expectedJoinCode: validToken,
		},
		{
			name:             "success - spectator",
			buildStubs:       func(store *mockdb.MockStore) {},
			query:            url.Values{"user_id": {validUserID.String()}, "server_id": {validServerID.String()}, "role": {"spectator"}},
			authHeader:       fmt.Sprintf("%s %s", commonMiddleware.AuthorizationBearer, validToken),
			expectedStatus:   http.StatusOK,
			expectedErr:      nil,
			expectedRole:     Spectator,
			expectedUserID:   validUserID,
			expectedServerID: validServerID,
			expectedJoinCode: validToken,
		},
		{
			name:           "missing user id",
			buildStubs:     func(store *mockdb.MockStore) {},
			query:          url.Values{"server_id": {validServerID.String()}},
			authHeader:     fmt.Sprintf("%s %s", commonMiddleware.AuthorizationBearer, validToken),
			expectedStatus: http.StatusBadRequest,
			expectedErr:    ErrMissingUserID,
		},
		{
			name:           "missing server id",
			buildStubs:     func(store *mockdb.MockStore) {},
			query:          url.Values{"user_id": {validUserID.String()}},
			authHeader:     fmt.Sprintf("%s %s", commonMiddleware.AuthorizationBearer, validToken),
			expectedStatus: http.StatusBadRequest,
			expectedErr:    ErrMissingServerID,
		},
		{
			name:           "invalid user id",
			buildStubs:     func(store *mockdb.MockStore) {},
			query:          url.Values{"server_id": {validServerID.String()}, "user_id": {"fake-user-id"}},
			authHeader:     fmt.Sprintf("%s %s", commonMiddleware.AuthorizationBearer, validToken),
			expectedStatus: http.StatusBadRequest,
			expectedErr:    ErrInvalidUserID,
		},
		{
			name:           "invalid server id",
			buildStubs:     func(store *mockdb.MockStore) {},
			query:          url.Values{"user_id": {validUserID.String()}, "server_id": {"fake-server-id"}},
			authHeader:     fmt.Sprintf("%s %s", commonMiddleware.AuthorizationBearer, validToken),
			expectedStatus: http.StatusBadRequest,
			expectedErr:    ErrInvalidServerID,
		},
		{
			name:           "failed to parse role",
			buildStubs:     func(store *mockdb.MockStore) {},
			query:          url.Values{"user_id": {validUserID.String()}, "server_id": {validServerID.String()}, "role": {"invalid"}},
			authHeader:     fmt.Sprintf("%s %s", commonMiddleware.AuthorizationBearer, validToken),
			expectedStatus: http.StatusBadRequest,
			expectedErr:    ErrInvalidRole,
		},
		{
			name: "failed to authorize moderator - internal server error",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetServerUserPermissions(gomock.Any(), db.GetServerUserPermissionsParams{
						ServerID: validServerID,
						UserID:   validUserID,
					}).
					Times(1).
					Return(db.GetServerUserPermissionsRow{}, errors.New("internal server error"))
			},
			query:          url.Values{"user_id": {validUserID.String()}, "server_id": {validServerID.String()}, "role": {"moderator"}},
			authHeader:     fmt.Sprintf("%s %s", commonMiddleware.AuthorizationBearer, validToken),
			expectedStatus: http.StatusInternalServerError,
			expectedErr:    ErrInternalServerError,
		},
		{
			name: "failed to authorize moderator - unauthorized",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetServerUserPermissions(gomock.Any(), db.GetServerUserPermissionsParams{
						ServerID: validServerID,
						UserID:   validUserID,
					}).
					Times(1).
					Return(db.GetServerUserPermissionsRow{
						HasElevatedPermissions: false,
					}, nil)
			},
			query:          url.Values{"user_id": {validUserID.String()}, "server_id": {validServerID.String()}, "role": {"moderator"}},
			authHeader:     fmt.Sprintf("%s %s", commonMiddleware.AuthorizationBearer, validToken),
			expectedStatus: http.StatusUnauthorized,
			expectedErr:    ErrUnauthorizedAccess,
		},
		{
			name:           "missing authorization header",
			buildStubs:     func(store *mockdb.MockStore) {},
			query:          url.Values{"user_id": {validUserID.String()}, "server_id": {validServerID.String()}},
			authHeader:     "",
			expectedStatus: http.StatusUnauthorized,
			expectedErr:    ErrMissingAuthorizationHeader,
		},
		{
			name:           "invalid authorization header format",
			buildStubs:     func(store *mockdb.MockStore) {},
			query:          url.Values{"user_id": {validUserID.String()}, "server_id": {validServerID.String()}},
			authHeader:     fmt.Sprintf("invalid %s", validToken),
			expectedStatus: http.StatusUnauthorized,
			expectedErr:    ErrInvalidAuthorizationHeader,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)

			wsHandler := &WsHandler{
				Store: store,
			}

			tc.buildStubs(store)

			req := httptest.NewRequest(http.MethodGet, "/ws?"+tc.query.Encode(), nil)
			if tc.authHeader != "" {
				req.Header.Set(commonMiddleware.AuthorizationHeader, tc.authHeader)
			}
			rec := httptest.NewRecorder()

			err := wsHandler.extractRequestMetadata(context.Background(), rec, req)

			if tc.expectedErr != nil {
				require.Error(t, err)
				require.Equal(t, tc.expectedErr, err)
				require.Equal(t, tc.expectedStatus, rec.Code)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.expectedRole, wsHandler.role)
			require.Equal(t, tc.expectedUserID, wsHandler.userID)
			require.Equal(t, tc.expectedServerID, wsHandler.serverID)
			require.Equal(t, tc.expectedJoinCode, wsHandler.serverJoinCode)
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

			wsHandler := &WsHandler{
				Config: cfg,
			}
			checkFn := wsHandler.checkOrigin(userId)
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
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			config := util.Config{
				ServiceName: "test",
			}

			cache := mockcache.NewMockCache(ctrl)
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
					Get(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(ctx context.Context, key string, dest *string) error {
						*dest = "valid-server-join-code"
						return nil
					})

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
					Get(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(ctx context.Context, key string, dest *string) error {
						*dest = ""
						return errors.New("error retrieving server join code")
					})
			},
			expectedErr:      ErrInternalServerError,
			expectedJoinCode: "",
		},
		{
			name: "server join code not found in cache",
			buildStubs: func(cache *mockcache.MockCache) {
				cache.EXPECT().
					Get(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(ctx context.Context, key string, dest *string) error {
						*dest = ""
						return commonCache.ErrKeyNotFound
					})
			},
			expectedErr:      ErrUnauthorizedAccess,
			expectedJoinCode: "",
		},
		{
			name: "could not delete server join code from cache",
			buildStubs: func(cache *mockcache.MockCache) {
				cache.EXPECT().
					Get(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(ctx context.Context, key string, dest *string) error {
						*dest = "valid-server-join-code"
						return nil
					})

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
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			config := util.Config{
				ServiceName: "test",
			}

			cache := mockcache.NewMockCache(ctrl)
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

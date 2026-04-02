package game

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	mockcache "github.com/spazzle-io/spazzle-api/libs/common/cache/mock"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/api/handler"
	mockdb "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/mock"
	db "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/sqlc"
	mockeventbus "github.com/spazzle-io/spazzle-api/services/gameplay/internal/eventbus/mock"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow"
	mockgameflowclient "github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow/mock"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow/types"
	mockservices "github.com/spazzle-io/spazzle-api/services/gameplay/internal/services/mock"
	authPb "github.com/spazzle-io/spazzle-api/services/proto/auth/auth/v1"
	pb "github.com/spazzle-io/spazzle-api/services/proto/gameplay/gameplay/v1"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestJoinGame(t *testing.T) {
	testCases := []struct {
		name          string
		req           *pb.JoinGameRequest
		buildStubs    func(bus *mockeventbus.MockEventBus, session *mockeventbus.MockSession, store *mockdb.MockStore, cache *mockcache.MockCache, gfClient *mockgameflowclient.MockClient, authService *mockservices.MockAuthGrpcService)
		checkResponse func(t *testing.T, res *pb.JoinGameResponse, err error)
	}{
		{
			name: "success - player",
			req: &pb.JoinGameRequest{
				ServerId: uuid.NewString(),
				Role:     pb.GameRole_GAME_ROLE_PLAYER,
			},
			buildStubs: func(bus *mockeventbus.MockEventBus, session *mockeventbus.MockSession, store *mockdb.MockStore, cache *mockcache.MockCache, gfClient *mockgameflowclient.MockClient, authService *mockservices.MockAuthGrpcService) {
				store.EXPECT().
					GetServerById(gomock.Any(), gomock.Any()).
					Times(2).
					Return(db.Server{
						ID:         uuid.New(),
						IsArchived: false,
						StakePerGame: pgtype.Numeric{
							Int:   big.NewInt(12),
							Valid: true,
						},
						NumRoundsPerGame:  10,
						RoundDurationSecs: 60,
					}, nil)

				authService.EXPECT().
					VerifyAccessToken(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(&authPb.VerifyAccessTokenResponse{
						AccessTokenPayload: &authPb.AccessTokenPayload{
							UserId: uuid.New().String(),
						},
					}, nil)

				store.EXPECT().
					GetServerUserPermissions(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.GetServerUserPermissionsRow{
						HasElevatedPermissions: false,
					}, nil)

				gfClient.EXPECT().
					Game(gomock.Any(), gomock.Any()).
					Times(1).
					Return(uuid.New(), nil)

				gfClient.EXPECT().
					HeartbeatGameServerInstance(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)

				gfClient.EXPECT().
					GetGameState(gomock.Any()).
					Times(1).
					Return(&types.GameStateView{
						GameID: uuid.New(),
					}, nil)

				bus.EXPECT().
					Session(gomock.Any()).
					Times(1).
					Return(session, nil)

				session.EXPECT().
					Subscribe(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(2).
					Return(nil)

				cache.EXPECT().
					Set(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)
			},
			checkResponse: func(t *testing.T, res *pb.JoinGameResponse, err error) {
				require.NoError(t, err)
				require.NotEmpty(t, res)

				require.Equal(t, pb.JoinGameStatus_JOIN_GAME_STATUS_SUCCESS, res.Status)
				require.Empty(t, res.RetryAfterMs)

				require.NotNil(t, res.UserId)
				require.NotEmpty(t, res.GameId)
				require.NotEmpty(t, res.JoinCode)
				require.NotNil(t, res.JoinCodeExpiresAt)
			},
		},
		{
			name: "success - moderator",
			req: &pb.JoinGameRequest{
				ServerId: uuid.NewString(),
				Role:     pb.GameRole_GAME_ROLE_MODERATOR,
			},
			buildStubs: func(bus *mockeventbus.MockEventBus, session *mockeventbus.MockSession, store *mockdb.MockStore, cache *mockcache.MockCache, gfClient *mockgameflowclient.MockClient, authService *mockservices.MockAuthGrpcService) {
				store.EXPECT().
					GetServerById(gomock.Any(), gomock.Any()).
					Times(2).
					Return(db.Server{
						ID:         uuid.New(),
						IsArchived: false,
						StakePerGame: pgtype.Numeric{
							Int:   big.NewInt(12),
							Valid: true,
						},
						NumRoundsPerGame:  10,
						RoundDurationSecs: 60,
					}, nil)

				authService.EXPECT().
					VerifyAccessToken(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(&authPb.VerifyAccessTokenResponse{
						AccessTokenPayload: &authPb.AccessTokenPayload{
							UserId: uuid.New().String(),
						},
					}, nil)

				store.EXPECT().
					GetServerUserPermissions(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.GetServerUserPermissionsRow{
						HasElevatedPermissions: true,
					}, nil)

				gfClient.EXPECT().
					Game(gomock.Any(), gomock.Any()).
					Times(1).
					Return(uuid.New(), nil)

				gfClient.EXPECT().
					HeartbeatGameServerInstance(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)

				gfClient.EXPECT().
					GetGameState(gomock.Any()).
					Times(1).
					Return(&types.GameStateView{
						GameID: uuid.New(),
					}, nil)

				bus.EXPECT().
					Session(gomock.Any()).
					Times(1).
					Return(session, nil)

				session.EXPECT().
					Subscribe(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(2).
					Return(nil)

				cache.EXPECT().
					Set(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)
			},
			checkResponse: func(t *testing.T, res *pb.JoinGameResponse, err error) {
				require.NoError(t, err)
				require.NotEmpty(t, res)

				require.Equal(t, pb.JoinGameStatus_JOIN_GAME_STATUS_SUCCESS, res.Status)
				require.Empty(t, res.RetryAfterMs)

				require.NotNil(t, res.UserId)
				require.NotEmpty(t, res.GameId)
				require.NotEmpty(t, res.JoinCode)
				require.NotNil(t, res.JoinCodeExpiresAt)
			},
		},
		{
			name: "success - spectator",
			req: &pb.JoinGameRequest{
				ServerId: uuid.NewString(),
				Role:     pb.GameRole_GAME_ROLE_SPECTATOR,
			},
			buildStubs: func(bus *mockeventbus.MockEventBus, session *mockeventbus.MockSession, store *mockdb.MockStore, cache *mockcache.MockCache, gfClient *mockgameflowclient.MockClient, authService *mockservices.MockAuthGrpcService) {
				store.EXPECT().
					GetServerById(gomock.Any(), gomock.Any()).
					Times(2).
					Return(db.Server{
						ID:         uuid.New(),
						IsArchived: false,
						StakePerGame: pgtype.Numeric{
							Int:   big.NewInt(12),
							Valid: true,
						},
						NumRoundsPerGame:  10,
						RoundDurationSecs: 60,
					}, nil)

				gfClient.EXPECT().
					Game(gomock.Any(), gomock.Any()).
					Times(1).
					Return(uuid.New(), nil)

				gfClient.EXPECT().
					HeartbeatGameServerInstance(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)

				gfClient.EXPECT().
					GetGameState(gomock.Any()).
					Times(1).
					Return(&types.GameStateView{
						GameID: uuid.New(),
					}, nil)

				bus.EXPECT().
					Session(gomock.Any()).
					Times(1).
					Return(session, nil)

				session.EXPECT().
					Subscribe(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(2).
					Return(nil)

				cache.EXPECT().
					Set(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)
			},
			checkResponse: func(t *testing.T, res *pb.JoinGameResponse, err error) {
				require.NoError(t, err)
				require.NotEmpty(t, res)

				require.Equal(t, pb.JoinGameStatus_JOIN_GAME_STATUS_SUCCESS, res.Status)
				require.Empty(t, res.RetryAfterMs)

				require.NotNil(t, res.UserId)
				require.NotEmpty(t, res.GameId)
				require.NotEmpty(t, res.JoinCode)
				require.NotNil(t, res.JoinCodeExpiresAt)
			},
		},
		{
			name: "invalid request parameters",
			req: &pb.JoinGameRequest{
				ServerId: "invalid-server-id",
				Role:     pb.GameRole_GAME_ROLE_UNSPECIFIED,
			},
			buildStubs: func(bus *mockeventbus.MockEventBus, session *mockeventbus.MockSession, store *mockdb.MockStore, cache *mockcache.MockCache, gfClient *mockgameflowclient.MockClient, authService *mockservices.MockAuthGrpcService) {
			},
			checkResponse: func(t *testing.T, res *pb.JoinGameResponse, err error) {
				require.Error(t, err)
				require.Empty(t, res)

				expectedFieldViolations := []string{"serverId", "role"}
				handler.CheckInvalidRequestParams(t, err, expectedFieldViolations)
			},
		},
		{
			name: "archived server",
			req: &pb.JoinGameRequest{
				ServerId: uuid.NewString(),
				Role:     pb.GameRole_GAME_ROLE_MODERATOR,
			},
			buildStubs: func(bus *mockeventbus.MockEventBus, session *mockeventbus.MockSession, store *mockdb.MockStore, cache *mockcache.MockCache, gfClient *mockgameflowclient.MockClient, authService *mockservices.MockAuthGrpcService) {
				store.EXPECT().
					GetServerById(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Server{
						ID:         uuid.New(),
						IsArchived: true,
					}, nil)
			},
			checkResponse: func(t *testing.T, res *pb.JoinGameResponse, err error) {
				require.Error(t, err)
				require.Empty(t, res)
			},
		},
		{
			name: "could not verify access token",
			req: &pb.JoinGameRequest{
				ServerId: uuid.NewString(),
				Role:     pb.GameRole_GAME_ROLE_PLAYER,
			},
			buildStubs: func(bus *mockeventbus.MockEventBus, session *mockeventbus.MockSession, store *mockdb.MockStore, cache *mockcache.MockCache, gfClient *mockgameflowclient.MockClient, authService *mockservices.MockAuthGrpcService) {
				store.EXPECT().
					GetServerById(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Server{
						ID:         uuid.New(),
						IsArchived: false,
					}, nil)

				authService.EXPECT().
					VerifyAccessToken(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(&authPb.VerifyAccessTokenResponse{}, errors.New("could not verify access token"))
			},
			checkResponse: func(t *testing.T, res *pb.JoinGameResponse, err error) {
				require.Error(t, err)
				require.Empty(t, res)
			},
		},
		{
			name: "could not fetch server user permissions",
			req: &pb.JoinGameRequest{
				ServerId: uuid.NewString(),
				Role:     pb.GameRole_GAME_ROLE_MODERATOR,
			},
			buildStubs: func(bus *mockeventbus.MockEventBus, session *mockeventbus.MockSession, store *mockdb.MockStore, cache *mockcache.MockCache, gfClient *mockgameflowclient.MockClient, authService *mockservices.MockAuthGrpcService) {
				store.EXPECT().
					GetServerById(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Server{
						ID:         uuid.New(),
						IsArchived: false,
					}, nil)

				authService.EXPECT().
					VerifyAccessToken(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(&authPb.VerifyAccessTokenResponse{
						AccessTokenPayload: &authPb.AccessTokenPayload{
							UserId: uuid.New().String(),
						},
					}, nil)

				store.EXPECT().
					GetServerUserPermissions(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.GetServerUserPermissionsRow{}, errors.New("could not fetch server user permissions"))
			},
			checkResponse: func(t *testing.T, res *pb.JoinGameResponse, err error) {
				require.Error(t, err)
				require.Empty(t, res)
			},
		},
		{
			name: "user does not have elevated server permissions",
			req: &pb.JoinGameRequest{
				ServerId: uuid.NewString(),
				Role:     pb.GameRole_GAME_ROLE_MODERATOR,
			},
			buildStubs: func(bus *mockeventbus.MockEventBus, session *mockeventbus.MockSession, store *mockdb.MockStore, cache *mockcache.MockCache, gfClient *mockgameflowclient.MockClient, authService *mockservices.MockAuthGrpcService) {
				store.EXPECT().
					GetServerById(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Server{
						ID:         uuid.New(),
						IsArchived: false,
					}, nil)

				authService.EXPECT().
					VerifyAccessToken(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(&authPb.VerifyAccessTokenResponse{
						AccessTokenPayload: &authPb.AccessTokenPayload{
							UserId: uuid.New().String(),
						},
					}, nil)

				store.EXPECT().
					GetServerUserPermissions(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.GetServerUserPermissionsRow{
						HasElevatedPermissions: false,
					}, nil)
			},
			checkResponse: func(t *testing.T, res *pb.JoinGameResponse, err error) {
				require.Error(t, err)
				require.Empty(t, res)
			},
		},
		{
			name: "game is ending",
			req: &pb.JoinGameRequest{
				ServerId: uuid.NewString(),
				Role:     pb.GameRole_GAME_ROLE_PLAYER,
			},
			buildStubs: func(bus *mockeventbus.MockEventBus, session *mockeventbus.MockSession, store *mockdb.MockStore, cache *mockcache.MockCache, gfClient *mockgameflowclient.MockClient, authService *mockservices.MockAuthGrpcService) {
				store.EXPECT().
					GetServerById(gomock.Any(), gomock.Any()).
					Times(2).
					Return(db.Server{
						ID:         uuid.New(),
						IsArchived: false,
						StakePerGame: pgtype.Numeric{
							Int:   big.NewInt(12),
							Valid: true,
						},
						NumRoundsPerGame:  10,
						RoundDurationSecs: 60,
					}, nil)

				authService.EXPECT().
					VerifyAccessToken(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(&authPb.VerifyAccessTokenResponse{
						AccessTokenPayload: &authPb.AccessTokenPayload{
							UserId: uuid.New().String(),
						},
					}, nil)

				store.EXPECT().
					GetServerUserPermissions(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.GetServerUserPermissionsRow{
						HasElevatedPermissions: false,
					}, nil)

				gfClient.EXPECT().
					Game(gomock.Any(), gomock.Any()).
					Times(1).
					Return(uuid.Nil, gameflow.ErrGameEnding)
			},
			checkResponse: func(t *testing.T, res *pb.JoinGameResponse, err error) {
				require.NoError(t, err)
				require.NotEmpty(t, res)

				require.Equal(t, pb.JoinGameStatus_JOIN_GAME_STATUS_GAME_ENDING, res.Status)
				require.Equal(t, int64(retryAfterGameEndingMs), res.RetryAfterMs)

				require.Empty(t, res.UserId)
				require.Empty(t, res.GameId)
				require.Empty(t, res.JoinCode)
				require.Empty(t, res.JoinCodeExpiresAt)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			deps := newTestDeps(t)

			tc.buildStubs(deps.bus, deps.session, deps.store, deps.cache, deps.gfClient, deps.authService)

			h := newTestHandler(deps)

			res, err := h.JoinGame(context.Background(), tc.req)
			tc.checkResponse(t, res, err)
		})
	}
}

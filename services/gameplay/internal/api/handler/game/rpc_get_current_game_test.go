package game

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/eventbus"
	mockeventbus "github.com/spazzle-io/spazzle-api/services/gameplay/internal/eventbus/mock"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameevents"

	"github.com/google/uuid"
	mockcache "github.com/spazzle-io/spazzle-api/libs/common/cache/mock"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/api/handler"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gamecache"
	mockgameflowclient "github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow/mock"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow/types"
	pb "github.com/spazzle-io/spazzle-api/services/proto/gameplay/gameplay/v1"
	"github.com/stretchr/testify/require"
	"go.temporal.io/api/serviceerror"
	"go.uber.org/mock/gomock"
)

func TestGetCurrentGame(t *testing.T) {
	testCases := []struct {
		name          string
		req           *pb.GetCurrentGameRequest
		buildStubs    func(gfClient *mockgameflowclient.MockClient, cache *mockcache.MockCache, bus *mockeventbus.MockEventBus)
		checkResponse func(t *testing.T, res *pb.GetCurrentGameResponse, err error)
	}{
		{
			name: "success - cached game",
			req: &pb.GetCurrentGameRequest{
				ServerId: uuid.New().String(),
			},
			buildStubs: func(gfClient *mockgameflowclient.MockClient, cache *mockcache.MockCache, bus *mockeventbus.MockEventBus) {
				cache.EXPECT().
					Get(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(ctx context.Context, key string, dest *pb.GetCurrentGameResponse) error {
						*dest = pb.GetCurrentGameResponse{
							Game: &pb.CurrentGameInfo{
								Id: uuid.New().String(),
							},
							LastCompletedRound: &pb.RoundSummary{
								Round: 2,
							},
						}
						return nil
					})
			},
			checkResponse: func(t *testing.T, res *pb.GetCurrentGameResponse, err error) {
				require.NoError(t, err)
				require.NotEmpty(t, res.GetGame())
				require.NotEmpty(t, res.GetLastCompletedRound())
			},
		},
		{
			name: "success - cache miss",
			req: &pb.GetCurrentGameRequest{
				ServerId: uuid.New().String(),
			},
			buildStubs: func(gfClient *mockgameflowclient.MockClient, cache *mockcache.MockCache, bus *mockeventbus.MockEventBus) {
				cache.EXPECT().
					Get(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(ctx context.Context, key string, dest *pb.GetCurrentGameResponse) error {
						*dest = pb.GetCurrentGameResponse{}
						return gamecache.ErrCurrentGameCacheMiss
					})

				gfClient.EXPECT().
					GetGameState(gomock.Any()).
					Times(1).
					Return(&types.GameStateView{
						GameID:       uuid.New(),
						CurrentRound: 2,
					}, nil)

				payload := gameevents.RoundEndedPayload{Word: "some-word"}
				marshalledPayload, err := json.Marshal(payload)
				require.NoError(t, err)

				bus.EXPECT().
					GetMarkerMessage(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(&eventbus.Message{
						ID:      "marker-msg",
						Payload: marshalledPayload,
					}, nil)

				cache.EXPECT().
					Set(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)
			},
			checkResponse: func(t *testing.T, res *pb.GetCurrentGameResponse, err error) {
				require.NoError(t, err)
				require.NotEmpty(t, res.GetGame())
				require.NotEmpty(t, res.GetLastCompletedRound())
			},
		},
		{
			name: "invalid request parameters",
			req:  &pb.GetCurrentGameRequest{},
			buildStubs: func(gfClient *mockgameflowclient.MockClient, cache *mockcache.MockCache, bus *mockeventbus.MockEventBus) {
			},
			checkResponse: func(t *testing.T, res *pb.GetCurrentGameResponse, err error) {
				require.Error(t, err)
				require.Empty(t, res.GetGame())

				expectedFieldViolations := []string{"serverId"}
				handler.CheckInvalidRequestParams(t, err, expectedFieldViolations)
			},
		},
		{
			name: "could not get game state - unknown error",
			req: &pb.GetCurrentGameRequest{
				ServerId: uuid.New().String(),
			},
			buildStubs: func(gfClient *mockgameflowclient.MockClient, cache *mockcache.MockCache, bus *mockeventbus.MockEventBus) {
				cache.EXPECT().
					Get(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(ctx context.Context, key string, dest *pb.GetCurrentGameResponse) error {
						*dest = pb.GetCurrentGameResponse{}
						return gamecache.ErrCurrentGameCacheMiss
					})

				gfClient.EXPECT().
					GetGameState(gomock.Any()).
					Times(1).
					Return(&types.GameStateView{
						GameID: uuid.New(),
					}, errors.New("could not get game state"))
			},
			checkResponse: func(t *testing.T, res *pb.GetCurrentGameResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, handler.InternalServerError)
				require.Empty(t, res.GetGame())
			},
		},
		{
			name: "could not get game state - game not found",
			req: &pb.GetCurrentGameRequest{
				ServerId: uuid.New().String(),
			},
			buildStubs: func(gfClient *mockgameflowclient.MockClient, cache *mockcache.MockCache, bus *mockeventbus.MockEventBus) {
				cache.EXPECT().
					Get(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(ctx context.Context, key string, dest *pb.GetCurrentGameResponse) error {
						*dest = pb.GetCurrentGameResponse{}
						return gamecache.ErrCurrentGameCacheMiss
					})

				gfClient.EXPECT().
					GetGameState(gomock.Any()).
					Times(1).
					Return(&types.GameStateView{
						GameID: uuid.New(),
					}, serviceerror.NewNotFound("game not found"))
			},
			checkResponse: func(t *testing.T, res *pb.GetCurrentGameResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, handler.GameNotFoundError)
				require.Empty(t, res.GetGame())
			},
		},
		{
			name: "cache miss - round <= 1 - no previous round ended msg",
			req: &pb.GetCurrentGameRequest{
				ServerId: uuid.New().String(),
			},
			buildStubs: func(gfClient *mockgameflowclient.MockClient, cache *mockcache.MockCache, bus *mockeventbus.MockEventBus) {
				cache.EXPECT().
					Get(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(ctx context.Context, key string, dest *pb.GetCurrentGameResponse) error {
						*dest = pb.GetCurrentGameResponse{}
						return gamecache.ErrCurrentGameCacheMiss
					})

				gfClient.EXPECT().
					GetGameState(gomock.Any()).
					Times(1).
					Return(&types.GameStateView{
						GameID:       uuid.New(),
						CurrentRound: 1,
					}, nil)

				cache.EXPECT().
					Set(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)
			},
			checkResponse: func(t *testing.T, res *pb.GetCurrentGameResponse, err error) {
				require.NoError(t, err)
				require.NotEmpty(t, res.GetGame())
				require.Empty(t, res.GetLastCompletedRound())
			},
		},
		{
			name: "cache miss - game ended - no previous round ended msg",
			req: &pb.GetCurrentGameRequest{
				ServerId: uuid.New().String(),
			},
			buildStubs: func(gfClient *mockgameflowclient.MockClient, cache *mockcache.MockCache, bus *mockeventbus.MockEventBus) {
				cache.EXPECT().
					Get(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(ctx context.Context, key string, dest *pb.GetCurrentGameResponse) error {
						*dest = pb.GetCurrentGameResponse{}
						return gamecache.ErrCurrentGameCacheMiss
					})

				gfClient.EXPECT().
					GetGameState(gomock.Any()).
					Times(1).
					Return(&types.GameStateView{
						GameID:       uuid.New(),
						CurrentRound: 3,
						Phase:        types.PhaseEndGame,
					}, nil)

				cache.EXPECT().
					Set(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)
			},
			checkResponse: func(t *testing.T, res *pb.GetCurrentGameResponse, err error) {
				require.NoError(t, err)
				require.NotEmpty(t, res.GetGame())
				require.Empty(t, res.GetLastCompletedRound())
			},
		},
		{
			name: "cache miss - no previous round ended msg",
			req: &pb.GetCurrentGameRequest{
				ServerId: uuid.New().String(),
			},
			buildStubs: func(gfClient *mockgameflowclient.MockClient, cache *mockcache.MockCache, bus *mockeventbus.MockEventBus) {
				cache.EXPECT().
					Get(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(ctx context.Context, key string, dest *pb.GetCurrentGameResponse) error {
						*dest = pb.GetCurrentGameResponse{}
						return gamecache.ErrCurrentGameCacheMiss
					})

				gfClient.EXPECT().
					GetGameState(gomock.Any()).
					Times(1).
					Return(&types.GameStateView{
						GameID:       uuid.New(),
						CurrentRound: 2,
					}, nil)

				bus.EXPECT().
					GetMarkerMessage(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil, nil)

				cache.EXPECT().
					Set(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)
			},
			checkResponse: func(t *testing.T, res *pb.GetCurrentGameResponse, err error) {
				require.NoError(t, err)
				require.NotEmpty(t, res.GetGame())
				require.Empty(t, res.GetLastCompletedRound())
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			deps := newTestDeps(t)

			tc.buildStubs(deps.gfClient, deps.cache, deps.bus)

			h := newTestHandler(deps)

			res, err := h.GetCurrentGame(context.Background(), tc.req)
			tc.checkResponse(t, res, err)
		})
	}
}

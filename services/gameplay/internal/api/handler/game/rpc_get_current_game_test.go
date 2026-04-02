package game

import (
	"context"
	"errors"
	"testing"

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
		buildStubs    func(gfClient *mockgameflowclient.MockClient, cache *mockcache.MockCache)
		checkResponse func(t *testing.T, res *pb.GetCurrentGameResponse, err error)
	}{
		{
			name: "success - cached game",
			req: &pb.GetCurrentGameRequest{
				ServerId: uuid.New().String(),
			},
			buildStubs: func(gfClient *mockgameflowclient.MockClient, cache *mockcache.MockCache) {
				cache.EXPECT().
					Get(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(ctx context.Context, key string, dest *pb.GetCurrentGameResponse) error {
						*dest = pb.GetCurrentGameResponse{
							Game: &pb.CurrentGameInfo{
								Id: uuid.New().String(),
							},
						}
						return nil
					})
			},
			checkResponse: func(t *testing.T, res *pb.GetCurrentGameResponse, err error) {
				require.NoError(t, err)
				require.NotEmpty(t, res.GetGame())
			},
		},
		{
			name: "success - cache miss",
			req: &pb.GetCurrentGameRequest{
				ServerId: uuid.New().String(),
			},
			buildStubs: func(gfClient *mockgameflowclient.MockClient, cache *mockcache.MockCache) {
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
					}, nil)

				cache.EXPECT().
					Set(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)
			},
			checkResponse: func(t *testing.T, res *pb.GetCurrentGameResponse, err error) {
				require.NoError(t, err)
				require.NotEmpty(t, res.GetGame())
			},
		},
		{
			name:       "invalid request parameters",
			req:        &pb.GetCurrentGameRequest{},
			buildStubs: func(gfClient *mockgameflowclient.MockClient, cache *mockcache.MockCache) {},
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
			buildStubs: func(gfClient *mockgameflowclient.MockClient, cache *mockcache.MockCache) {
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
			buildStubs: func(gfClient *mockgameflowclient.MockClient, cache *mockcache.MockCache) {
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
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			deps := newTestDeps(t)

			tc.buildStubs(deps.gfClient, deps.cache)

			h := newTestHandler(deps)

			res, err := h.GetCurrentGame(context.Background(), tc.req)
			tc.checkResponse(t, res, err)
		})
	}
}

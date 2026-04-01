package game

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	mockcache "github.com/spazzle-io/spazzle-api/libs/common/cache/mock"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/api/handler"
	mockdb "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/mock"
	db "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/sqlc"
	mockeventbus "github.com/spazzle-io/spazzle-api/services/gameplay/internal/eventbus/mock"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gamecache"
	mockgameflowclient "github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow/mock"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameserver"
	mockservices "github.com/spazzle-io/spazzle-api/services/gameplay/internal/services/mock"
	mockwordstore "github.com/spazzle-io/spazzle-api/services/gameplay/internal/wordstore/mock"
	pb "github.com/spazzle-io/spazzle-api/services/proto/gameplay/gameplay/v1"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestGetGame(t *testing.T) {
	testCases := []struct {
		name          string
		req           *pb.GetGameRequest
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, res *pb.GetGameResponse, err error)
	}{
		{
			name: "success",
			req: &pb.GetGameRequest{
				GameId: uuid.New().String(),
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetGameById(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Game{
						ID: uuid.New(),
						TotalPot: pgtype.Numeric{
							Valid: true,
						},
						GameStake: pgtype.Numeric{
							Valid: true,
						},
					}, nil)
			},
			checkResponse: func(t *testing.T, res *pb.GetGameResponse, err error) {
				require.NoError(t, err)
				require.NotEmpty(t, res.GetGame())
			},
		},
		{
			name: "invalid request parameters",
			req: &pb.GetGameRequest{
				GameId: "invalid",
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, res *pb.GetGameResponse, err error) {
				require.Error(t, err)
				require.Empty(t, res.GetGame())

				expectedFieldViolations := []string{"gameId"}
				handler.CheckInvalidRequestParams(t, err, expectedFieldViolations)
			},
		},
		{
			name: "could not fetch game - not found",
			req: &pb.GetGameRequest{
				GameId: uuid.New().String(),
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetGameById(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Game{}, db.RecordNotFoundError)
			},
			checkResponse: func(t *testing.T, res *pb.GetGameResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, handler.GameNotFoundError)
				require.Empty(t, res.GetGame())
			},
		},
		{
			name: "could not fetch game - unknown error",
			req: &pb.GetGameRequest{
				GameId: uuid.New().String(),
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetGameById(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Game{}, errors.New("unknown error"))
			},
			checkResponse: func(t *testing.T, res *pb.GetGameResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, handler.InternalServerError)
				require.Empty(t, res.GetGame())
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			bus := mockeventbus.NewMockEventBus(ctrl)
			cache := mockcache.NewMockCache(ctrl)
			gameCache := gamecache.New(getTestConfig(), cache)
			store := mockdb.NewMockStore(ctrl)
			gfClient := mockgameflowclient.NewMockClient(ctrl)
			wordStore := mockwordstore.NewMockStore(ctrl)
			gsManager := gameserver.NewManager()
			authService := mockservices.NewMockAuthGrpcService(ctrl)

			tc.buildStubs(store)

			gameHandler := newTestHandler(cache, gameCache, store, bus, gfClient, wordStore, gsManager, authService)

			res, err := gameHandler.GetGame(context.Background(), tc.req)
			tc.checkResponse(t, res, err)
		})
	}
}

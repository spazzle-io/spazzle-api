package game

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/api/handler"
	mockdb "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/mock"
	db "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/sqlc"
	pb "github.com/spazzle-io/spazzle-api/services/proto/gameplay/gameplay/v1"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestListServerGames(t *testing.T) {
	testCases := []struct {
		name          string
		req           *pb.ListServerGamesRequest
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, res *pb.ListServerGamesResponse, err error)
	}{
		{
			name: "success",
			req: &pb.ListServerGamesRequest{
				ServerId: uuid.NewString(),
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListServerGames(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.Game{
						{
							ID: uuid.New(),
							TotalPot: pgtype.Numeric{
								Valid: true,
							},
							GameStake: pgtype.Numeric{
								Valid: true,
							},
						},
					}, nil)

				store.EXPECT().
					GetTotalServerGamesCount(gomock.Any(), gomock.Any()).
					Times(1).
					Return(int64(9), nil)
			},
			checkResponse: func(t *testing.T, res *pb.ListServerGamesResponse, err error) {
				require.NoError(t, err)
				require.NotEmpty(t, res.GetGames())
				require.NotEmpty(t, res.GetCursor())
				require.Equal(t, int64(9), res.GetTotalCount())
			},
		},
		{
			name: "invalid request parameters",
			req: &pb.ListServerGamesRequest{
				ServerId: "invalid",
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, res *pb.ListServerGamesResponse, err error) {
				require.Error(t, err)
				require.Empty(t, res)

				expectedFieldViolations := []string{"serverId"}
				handler.CheckInvalidRequestParams(t, err, expectedFieldViolations)
			},
		},
		{
			name: "failed to fetch server games",
			req: &pb.ListServerGamesRequest{
				ServerId: uuid.NewString(),
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListServerGames(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.Game{}, errors.New("failed to fetch server games"))
			},
			checkResponse: func(t *testing.T, res *pb.ListServerGamesResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, handler.InternalServerError)
				require.Empty(t, res)
			},
		},
		{
			name: "failed to fetch server games count",
			req: &pb.ListServerGamesRequest{
				ServerId: uuid.NewString(),
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListServerGames(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.Game{
						{
							ID: uuid.New(),
							TotalPot: pgtype.Numeric{
								Valid: true,
							},
							GameStake: pgtype.Numeric{
								Valid: true,
							},
						},
					}, nil)

				store.EXPECT().
					GetTotalServerGamesCount(gomock.Any(), gomock.Any()).
					Times(1).
					Return(int64(0), errors.New("failed to fetch server games count"))
			},
			checkResponse: func(t *testing.T, res *pb.ListServerGamesResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, handler.InternalServerError)
				require.Empty(t, res)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			deps := newTestDeps(t)

			tc.buildStubs(deps.store)

			h := newTestHandler(deps)

			res, err := h.ListServerGames(context.Background(), tc.req)
			tc.checkResponse(t, res, err)
		})
	}
}

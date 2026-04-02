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

func TestListUserGames(t *testing.T) {
	testCases := []struct {
		name          string
		req           *pb.ListUserGamesRequest
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, res *pb.ListUserGamesResponse, err error)
	}{
		{
			name: "success",
			req: &pb.ListUserGamesRequest{
				UserId: uuid.NewString(),
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserGames(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.ListUserGamesRow{
						{
							UserID: uuid.New(),
							GameID: uuid.New(),
							Pnl: pgtype.Numeric{
								Valid: true,
							},
							TotalPot: pgtype.Numeric{
								Valid: true,
							},
							GameStake: pgtype.Numeric{
								Valid: true,
							},
							ProvisionalPayout: pgtype.Numeric{
								Valid: true,
							},
							TotalStakeLost: pgtype.Numeric{
								Valid: true,
							},
						},
					}, nil)

				store.EXPECT().
					GetTotalUserGamesCount(gomock.Any(), gomock.Any()).
					Times(1).
					Return(int64(2), nil)
			},
			checkResponse: func(t *testing.T, res *pb.ListUserGamesResponse, err error) {
				require.NoError(t, err)
				require.NotEmpty(t, res.GetGames())
				require.Equal(t, int64(2), res.GetTotalCount())
				require.NotEmpty(t, res.GetCursor())
			},
		},
		{
			name:       "invalid request parameters",
			req:        &pb.ListUserGamesRequest{},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, res *pb.ListUserGamesResponse, err error) {
				require.Error(t, err)
				require.Empty(t, res)

				expectedFieldViolations := []string{"userId"}
				handler.CheckInvalidRequestParams(t, err, expectedFieldViolations)
			},
		},
		{
			name: "failed to fetch user games",
			req: &pb.ListUserGamesRequest{
				UserId: uuid.NewString(),
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserGames(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.ListUserGamesRow{}, errors.New("failed to fetch user games"))
			},
			checkResponse: func(t *testing.T, res *pb.ListUserGamesResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, handler.InternalServerError)
				require.Empty(t, res)
			},
		},
		{
			name: "failed to fetch user games count",
			req: &pb.ListUserGamesRequest{
				UserId: uuid.NewString(),
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserGames(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.ListUserGamesRow{
						{
							UserID: uuid.New(),
							GameID: uuid.New(),
							Pnl: pgtype.Numeric{
								Valid: true,
							},
							TotalPot: pgtype.Numeric{
								Valid: true,
							},
							GameStake: pgtype.Numeric{
								Valid: true,
							},
							ProvisionalPayout: pgtype.Numeric{
								Valid: true,
							},
							TotalStakeLost: pgtype.Numeric{
								Valid: true,
							},
						},
					}, nil)

				store.EXPECT().
					GetTotalUserGamesCount(gomock.Any(), gomock.Any()).
					Times(1).
					Return(int64(0), errors.New("failed to fetch user games count"))
			},
			checkResponse: func(t *testing.T, res *pb.ListUserGamesResponse, err error) {
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

			res, err := h.ListUserGames(context.Background(), tc.req)
			tc.checkResponse(t, res, err)
		})
	}
}

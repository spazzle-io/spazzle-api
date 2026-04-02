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

func TestGetGameLeaderboard(t *testing.T) {
	gameID := uuid.New()

	testCases := []struct {
		name          string
		req           *pb.GetGameLeaderboardRequest
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, res *pb.GetGameLeaderboardResponse, err error)
	}{
		{
			name: "success",
			req: &pb.GetGameLeaderboardRequest{
				GameId: gameID.String(),
				Page:   3,
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetGameLeaderboard(gomock.Any(), gomock.Eq(db.GetGameLeaderboardParams{
						GameID:     gameID,
						PageOffset: 2 * leaderboardPageSize,
						PageSize:   leaderboardPageSize,
					})).
					Times(1).
					Return([]db.GamePlayer{
						{
							GameID: gameID,
							Pnl: pgtype.Numeric{
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
					GetTotalGamePlayersCount(gomock.Any(), gomock.Any()).
					Times(1).
					Return(int64(10), nil)
			},
			checkResponse: func(t *testing.T, res *pb.GetGameLeaderboardResponse, err error) {
				require.NoError(t, err)
				require.NotEmpty(t, res.GetPlayers())
				require.Equal(t, int64(10), res.GetTotalCount())
			},
		},
		{
			name:       "invalid request parameters",
			req:        &pb.GetGameLeaderboardRequest{},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, res *pb.GetGameLeaderboardResponse, err error) {
				require.Error(t, err)
				require.Empty(t, res.GetPlayers())
				require.Zero(t, res.GetTotalCount())

				expectedFieldViolations := []string{"gameId"}
				handler.CheckInvalidRequestParams(t, err, expectedFieldViolations)
			},
		},
		{
			name: "could not fetch game leaderboard",
			req: &pb.GetGameLeaderboardRequest{
				GameId: gameID.String(),
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetGameLeaderboard(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.GamePlayer{}, errors.New("could not fetch game leaderboard"))
			},
			checkResponse: func(t *testing.T, res *pb.GetGameLeaderboardResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, handler.InternalServerError)

				require.Empty(t, res.GetPlayers())
				require.Zero(t, res.GetTotalCount())
			},
		},
		{
			name: "could not fetch total player count",
			req: &pb.GetGameLeaderboardRequest{
				GameId: gameID.String(),
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetGameLeaderboard(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.GamePlayer{
						{
							GameID: gameID,
							Pnl: pgtype.Numeric{
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
					GetTotalGamePlayersCount(gomock.Any(), gomock.Any()).
					Times(1).
					Return(int64(0), errors.New("could not fetch total player count"))
			},
			checkResponse: func(t *testing.T, res *pb.GetGameLeaderboardResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, handler.InternalServerError)

				require.Empty(t, res.GetPlayers())
				require.Zero(t, res.GetTotalCount())
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			deps := newTestDeps(t)

			tc.buildStubs(deps.store)

			h := newTestHandler(deps)

			res, err := h.GetGameLeaderboard(context.Background(), tc.req)
			tc.checkResponse(t, res, err)
		})
	}
}

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

func TestGetUserStats(t *testing.T) {
	validUserID := uuid.New()

	testCases := []struct {
		name          string
		req           *pb.GetUserStatsRequest
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, res *pb.GetUserStatsResponse, err error)
	}{
		{
			name: "success",
			req: &pb.GetUserStatsRequest{
				UserId: validUserID.String(),
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetUserStats(gomock.Any(), gomock.Eq(validUserID)).
					Times(1).
					Return(db.UserStat{
						UserID: validUserID,
						TotalPnl: pgtype.Numeric{
							Valid: true,
						},
						TotalVolume: pgtype.Numeric{
							Valid: true,
						},
					}, nil)
			},
			checkResponse: func(t *testing.T, res *pb.GetUserStatsResponse, err error) {
				require.NoError(t, err)
				require.NotEmpty(t, res)
			},
		},
		{
			name:       "invalid request parameters",
			req:        &pb.GetUserStatsRequest{},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, res *pb.GetUserStatsResponse, err error) {
				require.Error(t, err)
				require.Empty(t, res)

				expectedFieldViolations := []string{"userId"}
				handler.CheckInvalidRequestParams(t, err, expectedFieldViolations)
			},
		},
		{
			name: "user stats not found",
			req: &pb.GetUserStatsRequest{
				UserId: validUserID.String(),
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetUserStats(gomock.Any(), gomock.Eq(validUserID)).
					Times(1).
					Return(db.UserStat{}, db.RecordNotFoundError)
			},
			checkResponse: func(t *testing.T, res *pb.GetUserStatsResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, handler.UserStatsNotFoundError)
				require.Empty(t, res)
			},
		},
		{
			name: "could not fetch user stats",
			req: &pb.GetUserStatsRequest{
				UserId: validUserID.String(),
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetUserStats(gomock.Any(), gomock.Eq(validUserID)).
					Times(1).
					Return(db.UserStat{}, errors.New("failed to fetch user stats"))
			},
			checkResponse: func(t *testing.T, res *pb.GetUserStatsResponse, err error) {
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

			res, err := h.GetUserStats(context.Background(), tc.req)
			tc.checkResponse(t, res, err)
		})
	}
}

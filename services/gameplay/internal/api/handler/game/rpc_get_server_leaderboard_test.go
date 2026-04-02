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
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gamecache"
	pb "github.com/spazzle-io/spazzle-api/services/proto/gameplay/gameplay/v1"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestGetServerLeaderboard(t *testing.T) {
	validServerID := uuid.New()

	testCases := []struct {
		name          string
		req           *pb.GetServerLeaderboardRequest
		buildStubs    func(store *mockdb.MockStore, cache *mockcache.MockCache)
		checkResponse func(t *testing.T, res *pb.GetServerLeaderboardResponse, err error)
	}{
		{
			name: "success - all time - default",
			req: &pb.GetServerLeaderboardRequest{
				ServerId: validServerID.String(),
			},
			buildStubs: func(store *mockdb.MockStore, cache *mockcache.MockCache) {
				store.EXPECT().
					GetServerById(gomock.Any(), gomock.Eq(validServerID)).
					Times(1).
					Return(db.Server{
						ID: validServerID,
					}, nil)

				store.EXPECT().
					GetServerLeaderboard(gomock.Any(), gomock.Eq(db.GetServerLeaderboardParams{
						ServerID:   validServerID,
						PageOffset: 0,
						PageSize:   leaderboardPageSize,
					})).
					Times(1).
					Return([]db.ServerPlayerStat{
						{
							UserID:   uuid.New(),
							ServerID: validServerID,
							TotalPnl: pgtype.Numeric{
								Valid: true,
							},
							TotalVolume: pgtype.Numeric{
								Valid: true,
							},
						},
					}, nil)

				store.EXPECT().
					GetTotalServerPlayerStatsCount(gomock.Any(), gomock.Eq(validServerID)).
					Times(1).
					Return(int64(5), nil)
			},
			checkResponse: func(t *testing.T, res *pb.GetServerLeaderboardResponse, err error) {
				require.NoError(t, err)

				require.NotEmpty(t, res.GetPlayers())
				require.Equal(t, int64(5), res.GetTotalCount())
				require.Equal(t, int32(1), res.GetPage())
				require.Equal(t, pb.TimeWindow_TIME_WINDOW_ALL_TIME, res.GetTimeWindow())
			},
		},
		{
			name: "success - all time - paginated",
			req: &pb.GetServerLeaderboardRequest{
				ServerId:   validServerID.String(),
				TimeWindow: pb.TimeWindow_TIME_WINDOW_ALL_TIME,
				Page:       2,
			},
			buildStubs: func(store *mockdb.MockStore, cache *mockcache.MockCache) {
				store.EXPECT().
					GetServerById(gomock.Any(), gomock.Eq(validServerID)).
					Times(1).
					Return(db.Server{
						ID: validServerID,
					}, nil)

				store.EXPECT().
					GetServerLeaderboard(gomock.Any(), gomock.Eq(db.GetServerLeaderboardParams{
						ServerID:   validServerID,
						PageOffset: leaderboardPageSize,
						PageSize:   leaderboardPageSize,
					})).
					Times(1).
					Return([]db.ServerPlayerStat{
						{
							UserID:   uuid.New(),
							ServerID: validServerID,
							TotalPnl: pgtype.Numeric{
								Valid: true,
							},
							TotalVolume: pgtype.Numeric{
								Valid: true,
							},
						},
					}, nil)

				store.EXPECT().
					GetTotalServerPlayerStatsCount(gomock.Any(), gomock.Eq(validServerID)).
					Times(1).
					Return(int64(25), nil)
			},
			checkResponse: func(t *testing.T, res *pb.GetServerLeaderboardResponse, err error) {
				require.NoError(t, err)

				require.NotEmpty(t, res.GetPlayers())
				require.Equal(t, int64(25), res.GetTotalCount())
				require.Equal(t, int32(2), res.GetPage())
				require.Equal(t, pb.TimeWindow_TIME_WINDOW_ALL_TIME, res.GetTimeWindow())
			},
		},
		{
			name: "success - windowed - cached",
			req: &pb.GetServerLeaderboardRequest{
				ServerId:   validServerID.String(),
				TimeWindow: pb.TimeWindow_TIME_WINDOW_TODAY,
				Page:       1,
			},
			buildStubs: func(store *mockdb.MockStore, cache *mockcache.MockCache) {
				store.EXPECT().
					GetServerById(gomock.Any(), gomock.Eq(validServerID)).
					Times(1).
					Return(db.Server{
						ID: validServerID,
					}, nil)

				cache.EXPECT().
					Get(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(ctx context.Context, key string, dest *pb.GetServerLeaderboardResponse) error {
						*dest = pb.GetServerLeaderboardResponse{
							Players: []*pb.LeaderboardEntry{
								{
									UserId: uuid.New().String(),
								},
							},
							TotalCount: 8,
							TimeWindow: pb.TimeWindow_TIME_WINDOW_TODAY,
							Page:       1,
						}
						return nil
					})
			},
			checkResponse: func(t *testing.T, res *pb.GetServerLeaderboardResponse, err error) {
				require.NoError(t, err)

				require.NotEmpty(t, res.GetPlayers())
				require.Equal(t, int64(8), res.GetTotalCount())
				require.Equal(t, int32(1), res.GetPage())
				require.Equal(t, pb.TimeWindow_TIME_WINDOW_TODAY, res.GetTimeWindow())
			},
		},
		{
			name: "success - windowed - cache miss",
			req: &pb.GetServerLeaderboardRequest{
				ServerId:   validServerID.String(),
				TimeWindow: pb.TimeWindow_TIME_WINDOW_WEEKLY,
			},
			buildStubs: func(store *mockdb.MockStore, cache *mockcache.MockCache) {
				store.EXPECT().
					GetServerById(gomock.Any(), gomock.Eq(validServerID)).
					Times(1).
					Return(db.Server{
						ID: validServerID,
					}, nil)

				cache.EXPECT().
					Get(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(ctx context.Context, key string, dest *pb.GetServerLeaderboardResponse) error {
						*dest = pb.GetServerLeaderboardResponse{}
						return gamecache.ErrLeaderboardCacheMiss
					})

				store.EXPECT().
					GetServerLeaderboardByWindow(gomock.Any(), gomock.Eq(db.GetServerLeaderboardByWindowParams{
						ServerID: validServerID,
						TimeWindow: pgtype.Interval{
							Days:  7,
							Valid: true,
						},
						PageOffset: 0,
						PageSize:   leaderboardPageSize,
					})).
					Times(1).
					Return([]db.GetServerLeaderboardByWindowRow{
						{
							UserID: uuid.New(),
							TotalPnl: pgtype.Numeric{
								Valid: true,
							},
							TotalVolume: pgtype.Numeric{
								Valid: true,
							},
						},
					}, nil)

				store.EXPECT().
					GetServerLeaderboardByWindowCount(gomock.Any(), gomock.Any()).
					Times(1).
					Return(int64(3), nil)

				cache.EXPECT().
					Set(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)
			},
			checkResponse: func(t *testing.T, res *pb.GetServerLeaderboardResponse, err error) {
				require.NoError(t, err)

				require.NotEmpty(t, res.GetPlayers())
				require.Equal(t, int64(3), res.GetTotalCount())
				require.Equal(t, int32(1), res.GetPage())
				require.Equal(t, pb.TimeWindow_TIME_WINDOW_WEEKLY, res.GetTimeWindow())
			},
		},
		{
			name: "invalid request parameters",
			req: &pb.GetServerLeaderboardRequest{
				ServerId: "invalid",
			},
			buildStubs: func(store *mockdb.MockStore, cache *mockcache.MockCache) {},
			checkResponse: func(t *testing.T, res *pb.GetServerLeaderboardResponse, err error) {
				require.Error(t, err)
				require.Empty(t, res.GetPlayers())
				require.Zero(t, res.GetTotalCount())

				expectedFieldViolations := []string{"serverId"}
				handler.CheckInvalidRequestParams(t, err, expectedFieldViolations)
			},
		},
		{
			name: "server not found",
			req: &pb.GetServerLeaderboardRequest{
				ServerId: validServerID.String(),
			},
			buildStubs: func(store *mockdb.MockStore, cache *mockcache.MockCache) {
				store.EXPECT().
					GetServerById(gomock.Any(), gomock.Eq(validServerID)).
					Times(1).
					Return(db.Server{}, db.RecordNotFoundError)
			},
			checkResponse: func(t *testing.T, res *pb.GetServerLeaderboardResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, handler.ServerNotFoundError)

				require.Empty(t, res.GetPlayers())
				require.Zero(t, res.GetTotalCount())
			},
		},
		{
			name: "failed to fetch all time server leaderboard",
			req: &pb.GetServerLeaderboardRequest{
				ServerId: validServerID.String(),
			},
			buildStubs: func(store *mockdb.MockStore, cache *mockcache.MockCache) {
				store.EXPECT().
					GetServerById(gomock.Any(), gomock.Eq(validServerID)).
					Times(1).
					Return(db.Server{
						ID: validServerID,
					}, nil)

				store.EXPECT().
					GetServerLeaderboard(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.ServerPlayerStat{}, errors.New("failed to fetch leaderboard"))
			},
			checkResponse: func(t *testing.T, res *pb.GetServerLeaderboardResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, handler.InternalServerError)

				require.Empty(t, res.GetPlayers())
				require.Zero(t, res.GetTotalCount())
			},
		},
		{
			name: "failed to fetch all time server leaderboard count",
			req: &pb.GetServerLeaderboardRequest{
				ServerId: validServerID.String(),
			},
			buildStubs: func(store *mockdb.MockStore, cache *mockcache.MockCache) {
				store.EXPECT().
					GetServerById(gomock.Any(), gomock.Eq(validServerID)).
					Times(1).
					Return(db.Server{
						ID: validServerID,
					}, nil)

				store.EXPECT().
					GetServerLeaderboard(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.ServerPlayerStat{
						{
							UserID:   uuid.New(),
							ServerID: validServerID,
							TotalPnl: pgtype.Numeric{
								Valid: true,
							},
							TotalVolume: pgtype.Numeric{
								Valid: true,
							},
						},
					}, nil)

				store.EXPECT().
					GetTotalServerPlayerStatsCount(gomock.Any(), gomock.Eq(validServerID)).
					Times(1).
					Return(int64(0), errors.New("failed to fetch leaderboard count"))
			},
			checkResponse: func(t *testing.T, res *pb.GetServerLeaderboardResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, handler.InternalServerError)

				require.Empty(t, res.GetPlayers())
				require.Zero(t, res.GetTotalCount())
			},
		},
		{
			name: "failed to fetch windowed server leaderboard",
			req: &pb.GetServerLeaderboardRequest{
				ServerId:   validServerID.String(),
				TimeWindow: pb.TimeWindow_TIME_WINDOW_TODAY,
			},
			buildStubs: func(store *mockdb.MockStore, cache *mockcache.MockCache) {
				store.EXPECT().
					GetServerById(gomock.Any(), gomock.Eq(validServerID)).
					Times(1).
					Return(db.Server{
						ID: validServerID,
					}, nil)

				cache.EXPECT().
					Get(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(ctx context.Context, key string, dest *pb.GetServerLeaderboardResponse) error {
						*dest = pb.GetServerLeaderboardResponse{}
						return gamecache.ErrLeaderboardCacheMiss
					})

				store.EXPECT().
					GetServerLeaderboardByWindow(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.GetServerLeaderboardByWindowRow{}, errors.New("failed to fetch windowed server leaderboard"))
			},
			checkResponse: func(t *testing.T, res *pb.GetServerLeaderboardResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, handler.InternalServerError)

				require.Empty(t, res.GetPlayers())
				require.Zero(t, res.GetTotalCount())
			},
		},
		{
			name: "failed to fetch windowed server leaderboard count",
			req: &pb.GetServerLeaderboardRequest{
				ServerId:   validServerID.String(),
				TimeWindow: pb.TimeWindow_TIME_WINDOW_TODAY,
			},
			buildStubs: func(store *mockdb.MockStore, cache *mockcache.MockCache) {
				store.EXPECT().
					GetServerById(gomock.Any(), gomock.Eq(validServerID)).
					Times(1).
					Return(db.Server{
						ID: validServerID,
					}, nil)

				cache.EXPECT().
					Get(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(ctx context.Context, key string, dest *pb.GetServerLeaderboardResponse) error {
						*dest = pb.GetServerLeaderboardResponse{}
						return gamecache.ErrLeaderboardCacheMiss
					})

				store.EXPECT().
					GetServerLeaderboardByWindow(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.GetServerLeaderboardByWindowRow{
						{
							UserID: uuid.New(),
							TotalPnl: pgtype.Numeric{
								Valid: true,
							},
							TotalVolume: pgtype.Numeric{
								Valid: true,
							},
						},
					}, nil)

				store.EXPECT().
					GetServerLeaderboardByWindowCount(gomock.Any(), gomock.Any()).
					Times(1).
					Return(int64(0), errors.New("failed to fetch windowed server leaderboard count"))
			},
			checkResponse: func(t *testing.T, res *pb.GetServerLeaderboardResponse, err error) {
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

			tc.buildStubs(deps.store, deps.cache)

			h := newTestHandler(deps)

			res, err := h.GetServerLeaderboard(context.Background(), tc.req)
			tc.checkResponse(t, res, err)
		})
	}
}

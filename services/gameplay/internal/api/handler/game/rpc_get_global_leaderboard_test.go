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

func TestGetGlobalLeaderboard(t *testing.T) {
	testCases := []struct {
		name          string
		req           *pb.GetGlobalLeaderboardRequest
		buildStubs    func(store *mockdb.MockStore, cache *mockcache.MockCache)
		checkResponse func(t *testing.T, res *pb.GetGlobalLeaderboardResponse, err error)
	}{
		{
			name: "success - time window not provided",
			req:  &pb.GetGlobalLeaderboardRequest{},
			buildStubs: func(store *mockdb.MockStore, cache *mockcache.MockCache) {
				store.EXPECT().
					GetGlobalLeaderboard(gomock.Any(), gomock.Eq(db.GetGlobalLeaderboardParams{
						PageOffset: 0,
						PageSize:   leaderboardPageSize,
					})).
					Times(1).
					Return([]db.UserStat{
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
					GetTotalUserStatsCount(gomock.Any()).
					Times(1).
					Return(int64(11), nil)
			},
			checkResponse: func(t *testing.T, res *pb.GetGlobalLeaderboardResponse, err error) {
				require.NoError(t, err)

				require.NotEmpty(t, res.GetPlayers())
				require.Equal(t, int64(11), res.GetTotalCount())
				require.Equal(t, int32(1), res.GetPage())
				require.Equal(t, pb.TimeWindow_TIME_WINDOW_ALL_TIME, res.GetTimeWindow())
			},
		},
		{
			name: "success - all time",
			req: &pb.GetGlobalLeaderboardRequest{
				TimeWindow: pb.TimeWindow_TIME_WINDOW_ALL_TIME,
				Page:       2,
			},
			buildStubs: func(store *mockdb.MockStore, cache *mockcache.MockCache) {
				store.EXPECT().
					GetGlobalLeaderboard(gomock.Any(), gomock.Eq(db.GetGlobalLeaderboardParams{
						PageOffset: leaderboardPageSize,
						PageSize:   leaderboardPageSize,
					})).
					Times(1).
					Return([]db.UserStat{
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
					GetTotalUserStatsCount(gomock.Any()).
					Times(1).
					Return(int64(11), nil)
			},
			checkResponse: func(t *testing.T, res *pb.GetGlobalLeaderboardResponse, err error) {
				require.NoError(t, err)

				require.NotEmpty(t, res.GetPlayers())
				require.Equal(t, int64(11), res.GetTotalCount())
				require.Equal(t, int32(2), res.GetPage())
				require.Equal(t, pb.TimeWindow_TIME_WINDOW_ALL_TIME, res.GetTimeWindow())
			},
		},
		{
			name: "success - windowed - cached",
			req: &pb.GetGlobalLeaderboardRequest{
				TimeWindow: pb.TimeWindow_TIME_WINDOW_TODAY,
				Page:       2,
			},
			buildStubs: func(store *mockdb.MockStore, cache *mockcache.MockCache) {
				cache.EXPECT().
					Get(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(ctx context.Context, key string, dest *pb.GetGlobalLeaderboardResponse) error {
						*dest = pb.GetGlobalLeaderboardResponse{
							Players: []*pb.LeaderboardEntry{
								{
									UserId: uuid.New().String(),
								},
							},
							TotalCount: 12,
							TimeWindow: pb.TimeWindow_TIME_WINDOW_TODAY,
							Page:       2,
						}
						return nil
					})
			},
			checkResponse: func(t *testing.T, res *pb.GetGlobalLeaderboardResponse, err error) {
				require.NoError(t, err)

				require.NotEmpty(t, res.GetPlayers())
				require.Equal(t, int64(12), res.GetTotalCount())
				require.Equal(t, int32(2), res.GetPage())
				require.Equal(t, pb.TimeWindow_TIME_WINDOW_TODAY, res.GetTimeWindow())
			},
		},
		{
			name: "success - windowed - cache miss",
			req: &pb.GetGlobalLeaderboardRequest{
				TimeWindow: pb.TimeWindow_TIME_WINDOW_MONTHLY,
				Page:       0,
			},
			buildStubs: func(store *mockdb.MockStore, cache *mockcache.MockCache) {
				cache.EXPECT().
					Get(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(ctx context.Context, key string, dest *pb.GetGlobalLeaderboardResponse) error {
						*dest = pb.GetGlobalLeaderboardResponse{}
						return gamecache.ErrLeaderboardCacheMiss
					})

				store.EXPECT().
					GetGlobalLeaderboardByWindow(gomock.Any(), gomock.Eq(db.GetGlobalLeaderboardByWindowParams{
						TimeWindow: pgtype.Interval{
							Months: 1,
							Valid:  true,
						},
						PageOffset: 0,
						PageSize:   leaderboardPageSize,
					})).
					Times(1).
					Return([]db.GetGlobalLeaderboardByWindowRow{
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
					GetGlobalLeaderboardByWindowCount(gomock.Any(), gomock.Any()).
					Times(1).
					Return(int64(11), nil)

				cache.EXPECT().
					Set(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)
			},
			checkResponse: func(t *testing.T, res *pb.GetGlobalLeaderboardResponse, err error) {
				require.NoError(t, err)
				require.NotEmpty(t, res.GetPlayers())
				require.Equal(t, int64(11), res.GetTotalCount())
				require.Equal(t, int32(1), res.GetPage())
				require.Equal(t, pb.TimeWindow_TIME_WINDOW_MONTHLY, res.GetTimeWindow())
			},
		},
		{
			name: "failed to fetch all time global leaderboard",
			req:  &pb.GetGlobalLeaderboardRequest{},
			buildStubs: func(store *mockdb.MockStore, cache *mockcache.MockCache) {
				store.EXPECT().
					GetGlobalLeaderboard(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.UserStat{}, errors.New("failed to fetch all time global leaderboard"))
			},
			checkResponse: func(t *testing.T, res *pb.GetGlobalLeaderboardResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, handler.InternalServerError)

				require.Empty(t, res.GetPlayers())
				require.Zero(t, res.GetTotalCount())
			},
		},
		{
			name: "failed to fetch all time global leaderboard count",
			req:  &pb.GetGlobalLeaderboardRequest{},
			buildStubs: func(store *mockdb.MockStore, cache *mockcache.MockCache) {
				store.EXPECT().
					GetGlobalLeaderboard(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.UserStat{
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
					GetTotalUserStatsCount(gomock.Any()).
					Times(1).
					Return(int64(0), errors.New("failed to fetch all time global leaderboard count"))
			},
			checkResponse: func(t *testing.T, res *pb.GetGlobalLeaderboardResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, handler.InternalServerError)

				require.Empty(t, res.GetPlayers())
				require.Zero(t, res.GetTotalCount())
			},
		},
		{
			name: "failed to fetch windowed leaderboard",
			req: &pb.GetGlobalLeaderboardRequest{
				TimeWindow: pb.TimeWindow_TIME_WINDOW_WEEKLY,
			},
			buildStubs: func(store *mockdb.MockStore, cache *mockcache.MockCache) {
				cache.EXPECT().
					Get(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(ctx context.Context, key string, dest *pb.GetGlobalLeaderboardResponse) error {
						*dest = pb.GetGlobalLeaderboardResponse{}
						return gamecache.ErrLeaderboardCacheMiss
					})

				store.EXPECT().
					GetGlobalLeaderboardByWindow(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.GetGlobalLeaderboardByWindowRow{}, errors.New("failed to fetch windowed global leaderboard"))
			},
			checkResponse: func(t *testing.T, res *pb.GetGlobalLeaderboardResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, handler.InternalServerError)

				require.Empty(t, res.GetPlayers())
				require.Zero(t, res.GetTotalCount())
			},
		},
		{
			name: "failed to fetch windowed leaderboard count",
			req: &pb.GetGlobalLeaderboardRequest{
				TimeWindow: pb.TimeWindow_TIME_WINDOW_WEEKLY,
			},
			buildStubs: func(store *mockdb.MockStore, cache *mockcache.MockCache) {
				cache.EXPECT().
					Get(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(ctx context.Context, key string, dest *pb.GetGlobalLeaderboardResponse) error {
						*dest = pb.GetGlobalLeaderboardResponse{}
						return gamecache.ErrLeaderboardCacheMiss
					})

				store.EXPECT().
					GetGlobalLeaderboardByWindow(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.GetGlobalLeaderboardByWindowRow{
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
					GetGlobalLeaderboardByWindowCount(gomock.Any(), gomock.Any()).
					Times(1).
					Return(int64(0), errors.New("failed to fetch windowed global leaderboard count"))
			},
			checkResponse: func(t *testing.T, res *pb.GetGlobalLeaderboardResponse, err error) {
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

			res, err := h.GetGlobalLeaderboard(context.Background(), tc.req)
			tc.checkResponse(t, res, err)
		})
	}
}

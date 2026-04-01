package db

import (
	"context"
	"math/big"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

func TestUpsertUserStats(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping db test in short mode")
	}

	userID := uuid.New()

	err := testStore.UpsertUserStats(context.Background(), UpsertUserStatsParams{
		UserID: userID,
		Score:  100,
		Pnl: pgtype.Numeric{
			Int:   big.NewInt(500000000000000),
			Valid: true,
		},
		Volume: pgtype.Numeric{
			Int:   big.NewInt(200000000000000),
			Valid: true,
		},
	})
	require.NoError(t, err)

	stats, err := testStore.GetUserStats(context.Background(), userID)
	require.NoError(t, err)
	require.Equal(t, int32(1), stats.TotalGames)
	require.Equal(t, int32(100), stats.TotalScore)
	require.True(t, stats.TotalPnl.Valid)
	require.True(t, stats.TotalVolume.Valid)

	// Upsert again — should increment
	err = testStore.UpsertUserStats(context.Background(), UpsertUserStatsParams{
		UserID: userID,
		Score:  50,
		Pnl: pgtype.Numeric{
			Int:   big.NewInt(300000000000000),
			Valid: true,
		},
		Volume: pgtype.Numeric{
			Int:   big.NewInt(200000000000000),
			Valid: true,
		},
	})
	require.NoError(t, err)

	stats, err = testStore.GetUserStats(context.Background(), userID)
	require.NoError(t, err)
	require.Equal(t, int32(2), stats.TotalGames)
	require.Equal(t, int32(150), stats.TotalScore)
	require.True(t, stats.TotalPnl.Valid)
	require.True(t, stats.TotalVolume.Valid)
}

func TestGetGlobalLeaderboard(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping db test in short mode")
	}

	for i := 0; i < 5; i++ {
		err := testStore.UpsertUserStats(context.Background(), UpsertUserStatsParams{
			UserID: uuid.New(),
			Score:  int32((i + 1) * 100),
			Pnl: pgtype.Numeric{
				Int:   big.NewInt(int64((i + 1) * 100000000000000)),
				Valid: true,
			},
			Volume: pgtype.Numeric{
				Int:   big.NewInt(200000000000000),
				Valid: true,
			},
		})
		require.NoError(t, err)
	}

	count, err := testStore.GetTotalUserStatsCount(context.Background())
	require.NoError(t, err)
	require.GreaterOrEqual(t, count, int64(5))

	leaderboard, err := testStore.GetGlobalLeaderboard(context.Background(), GetGlobalLeaderboardParams{
		PageSize: 3,
	})
	require.NoError(t, err)
	require.Len(t, leaderboard, 3)

	for i := 1; i < len(leaderboard); i++ {
		prev, err := leaderboard[i-1].TotalPnl.Float64Value()
		require.NoError(t, err)
		curr, err := leaderboard[i].TotalPnl.Float64Value()
		require.NoError(t, err)

		require.GreaterOrEqual(t, prev.Float64, curr.Float64)
	}
}

func TestGetGlobalLeaderboardByWindow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping db test in short mode")
	}

	for i := 0; i <= 2; i++ {
		server := createTestServer(t, uuid.New())
		game := createTestGame(t, server.ID)
		insertTestGamePlayers(t, game.ID, []uuid.UUID{uuid.New(), uuid.New()})
	}

	count, err := testStore.GetGlobalLeaderboardByWindowCount(context.Background(), pgtype.Interval{
		Microseconds: 1_000_000, // 1s,
		Valid:        true,
	})
	require.NoError(t, err)
	require.GreaterOrEqual(t, count, int64(6))

	leaderboard, err := testStore.GetGlobalLeaderboardByWindow(context.Background(), GetGlobalLeaderboardByWindowParams{
		TimeWindow: pgtype.Interval{
			Microseconds: 1_000_000, // 1s,
			Valid:        true,
		},
		PageSize:   4,
		PageOffset: 0,
	})
	require.NoError(t, err)
	require.Len(t, leaderboard, 4)

	for i := 1; i < len(leaderboard); i++ {
		prev, err := leaderboard[i-1].TotalPnl.Float64Value()
		require.NoError(t, err)
		curr, err := leaderboard[i].TotalPnl.Float64Value()
		require.NoError(t, err)

		require.GreaterOrEqual(t, prev.Float64, curr.Float64)
	}
}

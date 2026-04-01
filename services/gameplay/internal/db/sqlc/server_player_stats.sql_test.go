package db

import (
	"context"
	"math/big"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

func TestUpsertServerPlayerStats(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping db test in short mode")
	}

	server := createTestServer(t, uuid.New())
	userID := uuid.New()

	err := testStore.UpsertServerPlayerStats(context.Background(), UpsertServerPlayerStatsParams{
		ServerID: server.ID,
		UserID:   userID,
		Score:    100,
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

	// Upsert again — should increment
	err = testStore.UpsertServerPlayerStats(context.Background(), UpsertServerPlayerStatsParams{
		ServerID: server.ID,
		UserID:   userID,
		Score:    50,
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

	leaderboard, err := testStore.GetServerLeaderboard(context.Background(), GetServerLeaderboardParams{
		ServerID: server.ID,
		PageSize: 10,
	})
	require.NoError(t, err)
	require.Len(t, leaderboard, 1)
	require.Equal(t, int32(2), leaderboard[0].TotalGames)
	require.Equal(t, int32(150), leaderboard[0].TotalScore)
}

func TestGetServerLeaderboard(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping db test in short mode")
	}

	server := createTestServer(t, uuid.New())

	for i := 0; i < 5; i++ {
		err := testStore.UpsertServerPlayerStats(context.Background(), UpsertServerPlayerStatsParams{
			ServerID: server.ID,
			UserID:   uuid.New(),
			Score:    int32((i + 1) * 100),
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

	count, err := testStore.GetTotalServerPlayerStatsCount(context.Background(), server.ID)
	require.NoError(t, err)
	require.GreaterOrEqual(t, count, int64(5))

	leaderboard, err := testStore.GetServerLeaderboard(context.Background(), GetServerLeaderboardParams{
		ServerID: server.ID,
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

func TestGetServerLeaderboardByWindow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping db test in short mode")
	}

	server := createTestServer(t, uuid.New())

	for i := 0; i <= 2; i++ {
		game := createTestGame(t, server.ID)
		insertTestGamePlayers(t, game.ID, []uuid.UUID{uuid.New(), uuid.New()})
	}

	count, err := testStore.GetServerLeaderboardByWindowCount(context.Background(), GetServerLeaderboardByWindowCountParams{
		ServerID: server.ID,
		TimeWindow: pgtype.Interval{
			Microseconds: 1_000_000, // 1s,
			Valid:        true,
		},
	})
	require.NoError(t, err)
	require.GreaterOrEqual(t, count, int64(6))

	leaderboard, err := testStore.GetServerLeaderboardByWindow(context.Background(), GetServerLeaderboardByWindowParams{
		ServerID: server.ID,
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

package db

import (
	"context"
	"math/big"
	"testing"

	commonUtil "github.com/spazzle-io/spazzle-api/libs/common/util"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

func insertTestGamePlayers(t *testing.T, gameID uuid.UUID, playerIDs []uuid.UUID) {
	t.Helper()

	params := make([]InsertGamePlayersParams, 0, len(playerIDs))
	for i, playerID := range playerIDs {
		score, err := commonUtil.IntToInt32((len(playerIDs) - i) * 100)
		require.NoError(t, err)

		params = append(params, InsertGamePlayersParams{
			GameID: gameID,
			UserID: playerID,
			Score:  score,
			Pnl: pgtype.Numeric{
				Int:   big.NewInt(int64((len(playerIDs) - i) * 100000000000000)),
				Valid: true,
			},
			Position:     int32(i + 1),
			RoundsPlayed: 10,
			ProvisionalPayout: pgtype.Numeric{
				Int:   big.NewInt(int64((len(playerIDs) - i) * 300000000000000)),
				Valid: true,
			},
			TotalStakeLost: pgtype.Numeric{
				Int:   big.NewInt(int64((len(playerIDs) - i) * 200000000000000)),
				Valid: true,
			},
			IsEvicted: false,
		})
	}

	_, err := testStore.InsertGamePlayers(context.Background(), params)
	require.NoError(t, err)
}

func TestInsertGamePlayers(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping db test in short mode")
	}

	server := createTestServer(t, uuid.New())
	game := createTestGame(t, server.ID)

	playerIDs := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}
	insertTestGamePlayers(t, game.ID, playerIDs)

	count, err := testStore.GetTotalGamePlayersCount(context.Background(), game.ID)
	require.NoError(t, err)
	require.Equal(t, int64(3), count)
}

func TestGetGameLeaderboard(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping db test in short mode")
	}

	server := createTestServer(t, uuid.New())
	game := createTestGame(t, server.ID)

	playerIDs := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}
	insertTestGamePlayers(t, game.ID, playerIDs)

	leaderboard, err := testStore.GetGameLeaderboard(context.Background(), GetGameLeaderboardParams{
		GameID:   game.ID,
		PageSize: 10,
	})
	require.NoError(t, err)
	require.Len(t, leaderboard, 3)

	for i := 1; i < len(leaderboard); i++ {
		require.True(t, leaderboard[i-1].Position <= leaderboard[i].Position)
	}
}

func TestGetGameLeaderboardPagination(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping db test in short mode")
	}

	server := createTestServer(t, uuid.New())
	game := createTestGame(t, server.ID)

	playerIDs := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}
	insertTestGamePlayers(t, game.ID, playerIDs)

	firstPage, err := testStore.GetGameLeaderboard(context.Background(), GetGameLeaderboardParams{
		GameID:   game.ID,
		PageSize: 2,
	})
	require.NoError(t, err)
	require.Len(t, firstPage, 2)

	last := firstPage[len(firstPage)-1]
	secondPage, err := testStore.GetGameLeaderboard(context.Background(), GetGameLeaderboardParams{
		GameID:        game.ID,
		AfterPosition: pgtype.Int4{Int32: last.Position, Valid: true},
		AfterID:       pgtype.UUID{Bytes: last.UserID, Valid: true},
		PageSize:      2,
	})
	require.NoError(t, err)
	require.Len(t, secondPage, 1)
}

func TestGetTotalUserGamesCount(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping db test in short mode")
	}

	server := createTestServer(t, uuid.New())
	playerID := uuid.New()

	for i := 0; i < 3; i++ {
		game := createTestGame(t, server.ID)
		insertTestGamePlayers(t, game.ID, []uuid.UUID{playerID, uuid.New()})
	}

	count, err := testStore.GetTotalUserGamesCount(context.Background(), playerID)
	require.NoError(t, err)
	require.Equal(t, int64(3), count)
}

func TestListUserGames(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping db test in short mode")
	}

	server := createTestServer(t, uuid.New())
	playerID := uuid.New()

	for i := 0; i < 3; i++ {
		game := createTestGame(t, server.ID)
		insertTestGamePlayers(t, game.ID, []uuid.UUID{playerID, uuid.New()})
	}

	games, err := testStore.ListUserGames(context.Background(), ListUserGamesParams{
		UserID:   playerID,
		PageSize: 10,
	})
	require.NoError(t, err)
	require.Len(t, games, 3)

	for _, g := range games {
		require.Equal(t, playerID, g.UserID)
		require.Equal(t, server.ID, g.ServerID)
	}
}

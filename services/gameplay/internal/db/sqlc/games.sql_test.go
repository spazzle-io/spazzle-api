package db

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

func createTestGame(t *testing.T, serverID uuid.UUID) Game {
	t.Helper()

	params := CreateGameParams{
		ID:         uuid.New(),
		ServerID:   serverID,
		NumRounds:  10,
		NumPlayers: 2,
		TotalPot: pgtype.Numeric{
			Int:   big.NewInt(4000000000000000),
			Valid: true,
		},
		GameStake: pgtype.Numeric{
			Int:   big.NewInt(200000000000000),
			Valid: true,
		},
		StartedAt: time.Now().UTC().Add(-10 * time.Minute),
		EndedAt:   time.Now().UTC(),
	}

	game, err := testStore.CreateGame(context.Background(), params)
	require.NoError(t, err)
	require.NotEmpty(t, game)

	require.Equal(t, params.ID, game.ID)
	require.Equal(t, serverID, game.ServerID)
	require.Equal(t, params.NumRounds, game.NumRounds)
	require.Equal(t, params.NumPlayers, game.NumPlayers)
	require.True(t, game.TotalPot.Valid)
	require.True(t, game.GameStake.Valid)
	require.NotZero(t, game.StartedAt)
	require.NotZero(t, game.EndedAt)
	require.WithinDuration(t, time.Now().UTC(), game.CreatedAt, time.Second)

	return game
}

func TestCreateGame(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping db test in short mode")
	}

	server := createTestServer(t, uuid.New())
	game := createTestGame(t, server.ID)
	require.NotEmpty(t, game)
}

func TestGetGameById(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping db test in short mode")
	}

	server := createTestServer(t, uuid.New())
	game := createTestGame(t, server.ID)

	fetched, err := testStore.GetGameById(context.Background(), game.ID)
	require.NoError(t, err)
	require.Equal(t, game.ID, fetched.ID)
	require.Equal(t, game.ServerID, fetched.ServerID)
	require.Equal(t, game.NumRounds, fetched.NumRounds)
	require.Equal(t, game.NumPlayers, fetched.NumPlayers)
}

func TestListServerGames(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping db test in short mode")
	}

	server := createTestServer(t, uuid.New())

	for i := 0; i < 5; i++ {
		createTestGame(t, server.ID)
	}

	games, err := testStore.ListServerGames(context.Background(), ListServerGamesParams{
		ServerID: server.ID,
		PageSize: 3,
	})
	require.NoError(t, err)
	require.Len(t, games, 3)

	for i := 1; i < len(games); i++ {
		require.True(t, games[i-1].EndedAt.After(games[i].EndedAt) || games[i-1].EndedAt.Equal(games[i].EndedAt))
	}
}

func TestListServerGamesPagination(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping db test in short mode")
	}

	server := createTestServer(t, uuid.New())

	for i := 0; i < 5; i++ {
		createTestGame(t, server.ID)
	}

	firstPage, err := testStore.ListServerGames(context.Background(), ListServerGamesParams{
		ServerID: server.ID,
		PageSize: 3,
	})
	require.NoError(t, err)
	require.Len(t, firstPage, 3)

	last := firstPage[len(firstPage)-1]
	secondPage, err := testStore.ListServerGames(context.Background(), ListServerGamesParams{
		ServerID:     server.ID,
		AfterEndedAt: pgtype.Timestamptz{Time: last.EndedAt, Valid: true},
		AfterID:      pgtype.UUID{Bytes: last.ID, Valid: true},
		PageSize:     3,
	})
	require.NoError(t, err)
	require.Len(t, secondPage, 2)
}

func TestGetTotalServerGamesCount(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping db test in short mode")
	}

	server := createTestServer(t, uuid.New())

	for i := 0; i < 3; i++ {
		createTestGame(t, server.ID)
	}

	count, err := testStore.GetTotalServerGamesCount(context.Background(), server.ID)
	require.NoError(t, err)
	require.Equal(t, int64(3), count)
}

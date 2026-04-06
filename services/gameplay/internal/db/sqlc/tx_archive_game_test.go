package db

import (
	"context"
	"fmt"
	"testing"
	"time"

	commonUtil "github.com/spazzle-io/spazzle-api/libs/common/util"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestArchiveGameTx(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping db test in short mode")
	}

	server := createTestServer(t, uuid.New())

	player1 := uuid.New()
	player2 := uuid.New()
	player3 := uuid.New()

	params := ArchiveGameTxParams{
		GameID:    uuid.New(),
		ServerID:  server.ID,
		NumRounds: 10,
		TotalPot:  commonUtil.MustNewWei("6000000000000000"),
		GameStake: commonUtil.MustNewWei("200000000000000"),
		PlayerResults: []GamePlayerResult{
			{
				UserID:            player1,
				Score:             200,
				Pnl:               commonUtil.MustNewWei("-500000000000000"),
				Position:          2,
				RoundsPlayed:      10,
				ProvisionalPayout: commonUtil.MustNewWei("1500000000000000"),
				TotalStakeLost:    commonUtil.MustNewWei("2000000000000000"),
				IsEvicted:         false,
			},
			{
				UserID:            player2,
				Score:             300,
				Pnl:               commonUtil.MustNewWei("1000000000000000"),
				Position:          1,
				RoundsPlayed:      10,
				ProvisionalPayout: commonUtil.MustNewWei("3000000000000000"),
				TotalStakeLost:    commonUtil.MustNewWei("2000000000000000"),
				IsEvicted:         false,
			},
			{
				UserID:            player3,
				Score:             0,
				Pnl:               commonUtil.MustNewWei("-2000000000000000"),
				Position:          3,
				RoundsPlayed:      5,
				ProvisionalPayout: commonUtil.ZeroWei(),
				TotalStakeLost:    commonUtil.MustNewWei("2000000000000000"),
				IsEvicted:         true,
			},
		},
		StartedAt: time.Now().UTC().Add(-10 * time.Minute),
		EndedAt:   time.Now().UTC(),
	}

	result, err := testStore.ArchiveGameTx(context.Background(), params)
	require.NoError(t, err)
	require.NotEmpty(t, result.Game)

	game, err := testStore.GetGameById(context.Background(), params.GameID)
	require.NoError(t, err)
	require.Equal(t, params.GameID, game.ID)
	require.Equal(t, server.ID, game.ServerID)
	require.Equal(t, int32(10), game.NumRounds)
	require.Equal(t, int32(3), game.NumPlayers)

	count, err := testStore.GetTotalGamePlayersCount(context.Background(), params.GameID)
	require.NoError(t, err)
	require.Equal(t, int64(3), count)

	leaderboard, err := testStore.GetGameLeaderboard(context.Background(), GetGameLeaderboardParams{
		GameID:   params.GameID,
		PageSize: 10,
	})
	require.NoError(t, err)
	require.Len(t, leaderboard, 3)
	require.Equal(t, player2.String(), leaderboard[0].UserID.String())
	require.Equal(t, player1.String(), leaderboard[1].UserID.String())
	require.Equal(t, player3.String(), leaderboard[2].UserID.String())

	stats1, err := testStore.GetUserStats(context.Background(), player1)
	require.NoError(t, err)
	require.Equal(t, int32(1), stats1.TotalGames)
	require.Equal(t, int32(200), stats1.TotalScore)

	stats2, err := testStore.GetUserStats(context.Background(), player2)
	require.NoError(t, err)
	require.Equal(t, int32(1), stats2.TotalGames)
	require.Equal(t, int32(300), stats2.TotalScore)

	serverLeaderboard, err := testStore.GetServerLeaderboard(context.Background(), GetServerLeaderboardParams{
		ServerID: server.ID,
		PageSize: 10,
	})
	require.NoError(t, err)
	require.Len(t, serverLeaderboard, 3)

	updatedServer, err := testStore.GetServerById(context.Background(), server.ID)
	require.NoError(t, err)
	require.Equal(t, int32(1), updatedServer.TotalGames)
	require.Equal(t, int32(3), updatedServer.TotalPlayers)
}

func TestArchiveGameTxIdempotent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping db test in short mode")
	}

	server := createTestServer(t, uuid.New())

	params := ArchiveGameTxParams{
		GameID:    uuid.New(),
		ServerID:  server.ID,
		NumRounds: 5,
		TotalPot:  commonUtil.MustNewWei("2000000000000000"),
		GameStake: commonUtil.MustNewWei("200000000000000"),
		PlayerResults: []GamePlayerResult{
			{
				UserID:            uuid.New(),
				Score:             100,
				Pnl:               commonUtil.MustNewWei("500000000000000"),
				Position:          1,
				RoundsPlayed:      5,
				ProvisionalPayout: commonUtil.MustNewWei("1500000000000000"),
				TotalStakeLost:    commonUtil.MustNewWei("1000000000000000"),
				IsEvicted:         false,
			},
		},
		StartedAt: time.Now().UTC().Add(-5 * time.Minute),
		EndedAt:   time.Now().UTC(),
	}

	_, err := testStore.ArchiveGameTx(context.Background(), params)
	require.NoError(t, err)

	_, err = testStore.ArchiveGameTx(context.Background(), params)
	require.ErrorIs(t, err, ErrGameAlreadyExists)

	count, err := testStore.GetTotalServerGamesCount(context.Background(), server.ID)
	require.NoError(t, err)
	require.Equal(t, int64(1), count)

	updatedServer, err := testStore.GetServerById(context.Background(), server.ID)
	require.NoError(t, err)
	require.Equal(t, int32(1), updatedServer.TotalGames)
}

func TestArchiveGameTxMultipleGames(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping db test in short mode")
	}

	server := createTestServer(t, uuid.New())
	playerID := uuid.New()

	for i := 0; i < 3; i++ {
		params := ArchiveGameTxParams{
			GameID:    uuid.New(),
			ServerID:  server.ID,
			NumRounds: 5,
			TotalPot:  commonUtil.MustNewWei("2000000000000000"),
			GameStake: commonUtil.MustNewWei("200000000000000"),
			PlayerResults: []GamePlayerResult{
				{
					UserID:            playerID,
					Score:             int32((i + 1) * 100),
					Pnl:               commonUtil.MustNewWei(fmt.Sprintf("%d", (i+1)*100000000000000)),
					Position:          1,
					RoundsPlayed:      5,
					ProvisionalPayout: commonUtil.MustNewWei(fmt.Sprintf("%d", (i+1)*300000000000000)),
					TotalStakeLost:    commonUtil.MustNewWei(fmt.Sprintf("%d", (i+1)*200000000000000)),
					IsEvicted:         false,
				},
			},
			StartedAt: time.Now().UTC().Add(-time.Duration(10-i) * time.Minute),
			EndedAt:   time.Now().UTC().Add(-time.Duration(5-i) * time.Minute),
		}

		_, err := testStore.ArchiveGameTx(context.Background(), params)
		require.NoError(t, err)
	}

	updatedServer, err := testStore.GetServerById(context.Background(), server.ID)
	require.NoError(t, err)
	require.Equal(t, int32(3), updatedServer.TotalGames)
	require.Equal(t, int32(3), updatedServer.TotalPlayers)

	stats, err := testStore.GetUserStats(context.Background(), playerID)
	require.NoError(t, err)
	require.Equal(t, int32(3), stats.TotalGames)
	require.Equal(t, int32(600), stats.TotalScore) // 100 + 200 + 300

	serverLeaderboard, err := testStore.GetServerLeaderboard(context.Background(), GetServerLeaderboardParams{
		ServerID: server.ID,
		PageSize: 10,
	})
	require.NoError(t, err)
	require.Len(t, serverLeaderboard, 1)
	require.Equal(t, int32(3), serverLeaderboard[0].TotalGames)

	userGames, err := testStore.ListUserGames(context.Background(), ListUserGamesParams{
		UserID:   playerID,
		PageSize: 10,
	})
	require.NoError(t, err)
	require.Len(t, userGames, 3)
}

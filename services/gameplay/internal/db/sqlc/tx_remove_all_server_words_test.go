package db

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestRemoveAllServerWordsTx(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping db test in short mode")
	}

	server := createTestServer(t, uuid.New())
	require.NotEmpty(t, server)

	wordsToAdd := []string{"word-1", "word-2"}

	params := AddServerWordsTxParams{
		ServerId: server.ID,
		Words:    wordsToAdd,
	}
	txResult, err := testStore.AddServerWordsTx(context.Background(), params)
	require.NoError(t, err)
	require.Equal(t, int32(2), txResult.NumWordsAdded)

	testCases := []struct {
		name        string
		serverId    uuid.UUID
		checkResult func(t *testing.T, serverId uuid.UUID, txResult RemoveAllServerWordsTxResult, err error)
	}{
		{
			name:     "success",
			serverId: server.ID,
			checkResult: func(t *testing.T, serverId uuid.UUID, txResult RemoveAllServerWordsTxResult, err error) {
				require.NoError(t, err)

				updatedServer, err := testStore.GetServerById(context.Background(), serverId)
				require.NoError(t, err)
				require.NotEmpty(t, updatedServer)
				require.Zero(t, updatedServer.NumCustomWords)
			},
		},
		{
			name:     "server does not exist",
			serverId: uuid.New(),
			checkResult: func(t *testing.T, serverId uuid.UUID, txResult RemoveAllServerWordsTxResult, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, ErrServerNotfound.Error())
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			txResult, err := testStore.RemoveAllServerWordsTx(context.Background(), tc.serverId)
			tc.checkResult(t, tc.serverId, txResult, err)
		})
	}
}

package db

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestRemoveServerWordsTx(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping db test in short mode")
	}

	server := createTestServer(t, uuid.New())
	require.NotEmpty(t, server)

	testCases := []struct {
		name                     string
		serverId                 uuid.UUID
		expectServerToExist      bool
		wordsToRemove            []string
		expectedNumWordsToRemove int32
		checkResult              func(t *testing.T, serverId uuid.UUID, initialNumWords int32, expectedNumWordsToRemove int32, txResult RemoveServerWordsTxResult, err error)
	}{
		{
			name:                     "success",
			serverId:                 server.ID,
			expectServerToExist:      true,
			wordsToRemove:            []string{"word-1", "word-2"},
			expectedNumWordsToRemove: 2,
			checkResult: func(t *testing.T, serverId uuid.UUID, initialNumWords int32, expectedNumWordsToRemove int32, txResult RemoveServerWordsTxResult, err error) {
				require.NoError(t, err)
				require.Equal(t, int32(2), txResult.NumWordsRemoved)

				updatedServer, err := testStore.GetServerById(context.Background(), serverId)
				require.NoError(t, err)
				require.NotEmpty(t, updatedServer)
				require.Equal(t, initialNumWords-expectedNumWordsToRemove, updatedServer.NumCustomWords)
			},
		},
		{
			name:                     "remove words partially",
			serverId:                 server.ID,
			expectServerToExist:      true,
			wordsToRemove:            []string{"word-1"},
			expectedNumWordsToRemove: 1,
			checkResult: func(t *testing.T, serverId uuid.UUID, initialNumWords int32, expectedNumWordsToRemove int32, txResult RemoveServerWordsTxResult, err error) {
				require.NoError(t, err)
				require.Equal(t, int32(1), txResult.NumWordsRemoved)

				updatedServer, err := testStore.GetServerById(context.Background(), serverId)
				require.NoError(t, err)
				require.NotEmpty(t, updatedServer)
				require.Equal(t, initialNumWords-expectedNumWordsToRemove, updatedServer.NumCustomWords)
			},
		},
		{
			name:                "server does not exist",
			serverId:            uuid.New(),
			expectServerToExist: false,
			checkResult: func(t *testing.T, serverId uuid.UUID, initialNumWords int32, expectedNumWordsToRemove int32, txResult RemoveServerWordsTxResult, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, ErrServerNotfound.Error())
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			initialNumCustomWords := int32(0)
			if tc.expectServerToExist {
				wordsToAdd := []string{"word-1", "word-2"}

				params := AddServerWordsTxParams{
					ServerId: server.ID,
					Words:    wordsToAdd,
				}
				txResult, err := testStore.AddServerWordsTx(context.Background(), params)
				require.NoError(t, err)
				require.Equal(t, int32(2), txResult.NumWordsAdded)

				server, err := testStore.GetServerById(context.Background(), tc.serverId)
				require.NoError(t, err)

				initialNumCustomWords = server.NumCustomWords
			}

			params := RemoveServerWordsTxParams{
				ServerId: tc.serverId,
				Words:    tc.wordsToRemove,
			}
			txResult, err := testStore.RemoveServerWordsTx(context.Background(), params)
			tc.checkResult(t, tc.serverId, initialNumCustomWords, tc.expectedNumWordsToRemove, txResult, err)
		})
	}
}

package db

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestAddServerWordsTx(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping db test in short mode")
	}

	server := createTestServer(t, uuid.New())
	require.NotEmpty(t, server)

	testCases := []struct {
		name                  string
		serverId              uuid.UUID
		expectServerToExist   bool
		wordsToAdd            []string
		expectedNumWordsToAdd int32
		checkResult           func(t *testing.T, serverId uuid.UUID, expectedNumWordsToAdd int32, txResult AddServerWordsTxResult, err error)
	}{
		{
			name:                  "success",
			serverId:              server.ID,
			expectServerToExist:   true,
			wordsToAdd:            []string{"word-1", "word-2"},
			expectedNumWordsToAdd: 2,
			checkResult: func(t *testing.T, serverId uuid.UUID, expectedNumWordsToAdd int32, txResult AddServerWordsTxResult, err error) {
				require.NoError(t, err)
				require.Equal(t, int32(2), txResult.NumWordsAdded)

				updatedServer, err := testStore.GetServerById(context.Background(), serverId)
				require.NoError(t, err)
				require.NotEmpty(t, updatedServer)
				require.Equal(t, expectedNumWordsToAdd, updatedServer.NumCustomWords)
			},
		},
		{
			name:                  "add words sequentially",
			serverId:              server.ID,
			expectServerToExist:   true,
			wordsToAdd:            []string{"word-1", "word-2"},
			expectedNumWordsToAdd: 2,
			checkResult: func(t *testing.T, serverId uuid.UUID, expectedNumWordsToAdd int32, txResult AddServerWordsTxResult, err error) {
				require.NoError(t, err)
				require.Equal(t, int32(2), txResult.NumWordsAdded)

				// Add additional words
				params := AddServerWordsTxParams{
					ServerId: serverId,
					Words:    []string{"word-3", "word-4"},
				}
				txResult, err = testStore.AddServerWordsTx(context.Background(), params)
				require.NoError(t, err)
				require.Equal(t, int32(2), txResult.NumWordsAdded)

				updatedServer, err := testStore.GetServerById(context.Background(), serverId)
				require.NoError(t, err)
				require.NotEmpty(t, updatedServer)
				require.Equal(t, expectedNumWordsToAdd+2, updatedServer.NumCustomWords)
			},
		},
		{
			name:                  "add duplicate words",
			serverId:              server.ID,
			expectServerToExist:   true,
			wordsToAdd:            []string{"word-1", "word-2"},
			expectedNumWordsToAdd: 2,
			checkResult: func(t *testing.T, serverId uuid.UUID, expectedNumWordsToAdd int32, txResult AddServerWordsTxResult, err error) {
				require.NoError(t, err)
				require.Equal(t, int32(2), txResult.NumWordsAdded)

				// Add some duplicate words
				params := AddServerWordsTxParams{
					ServerId: serverId,
					Words:    []string{"word-2", "word-3"},
				}
				txResult, err = testStore.AddServerWordsTx(context.Background(), params)
				require.NoError(t, err)
				require.Equal(t, int32(1), txResult.NumWordsAdded)

				updatedServer, err := testStore.GetServerById(context.Background(), serverId)
				require.NoError(t, err)
				require.NotEmpty(t, updatedServer)
				require.Equal(t, expectedNumWordsToAdd+1, updatedServer.NumCustomWords)
			},
		},
		{
			name:                "server does not exist",
			serverId:            uuid.New(),
			expectServerToExist: false,
			checkResult: func(t *testing.T, serverId uuid.UUID, expectedNumWordsToAdd int32, txResult AddServerWordsTxResult, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, ErrServerNotfound.Error())
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.expectServerToExist {
				_, err := testStore.RemoveAllServerWordsTx(context.Background(), tc.serverId)
				require.NoError(t, err)
			}

			params := AddServerWordsTxParams{
				ServerId: tc.serverId,
				Words:    tc.wordsToAdd,
			}
			txResult, err := testStore.AddServerWordsTx(context.Background(), params)
			tc.checkResult(t, tc.serverId, tc.expectedNumWordsToAdd, txResult, err)
		})
	}
}

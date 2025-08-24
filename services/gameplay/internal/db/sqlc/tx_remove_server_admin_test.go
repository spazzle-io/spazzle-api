package db

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestRemoveServerAdminTx(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping db test in short mode")
	}

	server := createTestServer(t, uuid.New())
	require.NotEmpty(t, server)

	testCases := []struct {
		name        string
		serverId    uuid.UUID
		userId      uuid.UUID
		buildParams func(t *testing.T, serverId uuid.UUID, userId uuid.UUID) RemoveServerAdminTxParams
		checkResult func(t *testing.T, serverId uuid.UUID, userId uuid.UUID, err error)
	}{
		{
			name:     "success",
			serverId: server.ID,
			userId:   uuid.New(),
			buildParams: func(t *testing.T, serverId uuid.UUID, userId uuid.UUID) RemoveServerAdminTxParams {
				txResult, err := testStore.AddServerAdminTx(context.Background(), AddServerAdminTxParams{
					ServerId: serverId,
					UserId:   userId,
				})
				require.NoError(t, err)
				require.NotEmpty(t, txResult)

				return RemoveServerAdminTxParams{
					ServerId: serverId,
					UserId:   userId,
				}
			},
			checkResult: func(t *testing.T, serverId uuid.UUID, userId uuid.UUID, err error) {
				require.NoError(t, err)

				updatedServer, err := testStore.GetServerById(context.Background(), serverId)
				require.NoError(t, err)
				require.NotEmpty(t, updatedServer)
				require.Equal(t, int32(0), updatedServer.NumAdmins)
			},
		},
		{
			name:     "server does not exist",
			serverId: uuid.New(),
			userId:   uuid.New(),
			buildParams: func(t *testing.T, serverId uuid.UUID, userId uuid.UUID) RemoveServerAdminTxParams {
				return RemoveServerAdminTxParams{
					ServerId: serverId,
					UserId:   userId,
				}
			},
			checkResult: func(t *testing.T, serverId uuid.UUID, userId uuid.UUID, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, ErrServerNotfound.Error())
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			params := tc.buildParams(t, tc.serverId, tc.userId)
			err := testStore.RemoveServerAdminTx(context.Background(), params)
			tc.checkResult(t, tc.serverId, tc.userId, err)
		})
	}
}

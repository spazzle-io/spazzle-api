package db

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestAddServerAdminTx(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping db test in short mode")
	}

	server := createTestServer(t, uuid.New())
	require.NotEmpty(t, server)

	testCases := []struct {
		name        string
		serverId    uuid.UUID
		userId      uuid.UUID
		buildParams func(t *testing.T, serverId uuid.UUID, userId uuid.UUID) AddServerAdminTxParams
		checkResult func(t *testing.T, serverId uuid.UUID, userId uuid.UUID, txResult AddServerAdminTxResult, err error)
	}{
		{
			name:     "success",
			serverId: server.ID,
			userId:   uuid.New(),
			buildParams: func(t *testing.T, serverId uuid.UUID, userId uuid.UUID) AddServerAdminTxParams {
				return AddServerAdminTxParams{
					ServerId: serverId,
					UserId:   userId,
				}
			},
			checkResult: func(t *testing.T, serverId uuid.UUID, userId uuid.UUID, txResult AddServerAdminTxResult, err error) {
				require.NoError(t, err)
				require.Equal(t, serverId, txResult.ServerAdmin.ServerID)
				require.Equal(t, userId, txResult.ServerAdmin.UserID)
				require.NotZero(t, txResult.ServerAdmin.AddedAt)
				require.WithinDuration(t, time.Now().UTC(), txResult.ServerAdmin.AddedAt, time.Second)

				updatedServer, err := testStore.GetServerById(context.Background(), txResult.ServerAdmin.ServerID)
				require.NoError(t, err)
				require.NotEmpty(t, updatedServer)
				require.Equal(t, int32(1), updatedServer.NumAdmins)

				anotherTxResult, err := testStore.AddServerAdminTx(context.Background(), AddServerAdminTxParams{
					ServerId: serverId,
					UserId:   uuid.New(),
				})
				require.NoError(t, err)
				require.NotEmpty(t, anotherTxResult)

				updatedServer, err = testStore.GetServerById(context.Background(), txResult.ServerAdmin.ServerID)
				require.NoError(t, err)
				require.NotEmpty(t, updatedServer)
				require.Equal(t, int32(2), updatedServer.NumAdmins)
			},
		},
		{
			name:     "server does not exist",
			serverId: uuid.New(),
			userId:   uuid.New(),
			buildParams: func(t *testing.T, serverId uuid.UUID, userId uuid.UUID) AddServerAdminTxParams {
				return AddServerAdminTxParams{
					ServerId: serverId,
					UserId:   userId,
				}
			},
			checkResult: func(t *testing.T, serverId uuid.UUID, userId uuid.UUID, txResult AddServerAdminTxResult, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, ErrServerNotfound.Error())
				require.Empty(t, txResult)
			},
		},
		{
			name:     "server admin already exists",
			serverId: server.ID,
			userId:   uuid.New(),
			buildParams: func(t *testing.T, serverId uuid.UUID, userId uuid.UUID) AddServerAdminTxParams {
				txResult, err := testStore.AddServerAdminTx(context.Background(), AddServerAdminTxParams{
					ServerId: serverId,
					UserId:   userId,
				})
				require.NoError(t, err)
				require.NotEmpty(t, txResult)

				return AddServerAdminTxParams{
					ServerId: serverId,
					UserId:   userId,
				}
			},
			checkResult: func(t *testing.T, serverId uuid.UUID, userId uuid.UUID, txResult AddServerAdminTxResult, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, ErrUserAlreadyAdmin.Error())
				require.Empty(t, txResult)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			params := tc.buildParams(t, tc.serverId, tc.userId)
			txResult, err := testStore.AddServerAdminTx(context.Background(), params)
			tc.checkResult(t, tc.serverId, tc.userId, txResult, err)
		})
	}
}

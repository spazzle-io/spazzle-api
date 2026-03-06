package gameserver

import (
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/stretchr/testify/require"
)

func createTestGameServerManager(t *testing.T) *Manager {
	gameServerManager := NewManager()
	require.NotEmpty(t, gameServerManager)
	require.Empty(t, gameServerManager.gameServers)

	return gameServerManager
}

func TestCreateGameServerManager(t *testing.T) {
	gameServerManager := createTestGameServerManager(t)
	require.NotEmpty(t, gameServerManager)
}

func TestGetOrCreateGameServer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	sm := createTestGameServerManager(t)
	require.NotEmpty(t, sm)

	serverID := uuid.New()
	config := &Config{}

	gsCall1, err := sm.GetOrCreateGameServer(serverID, config)
	require.NoError(t, err)
	require.NotEmpty(t, gsCall1)

	gsCall2, err := sm.GetOrCreateGameServer(serverID, config)
	require.NoError(t, err)
	require.NotEmpty(t, gsCall2)

	require.Equal(t, gsCall1, gsCall2)
}

func TestRemoveGameServerIfClosed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	sm := createTestGameServerManager(t)
	require.NotEmpty(t, sm)

	serverID := uuid.New()
	config := &Config{}

	gs, err := sm.GetOrCreateGameServer(serverID, config)
	require.NoError(t, err)
	gs.shutdown()

	require.NotEmpty(t, sm.gameServers[serverID])

	sm.RemoveGameServerIfClosed(serverID)

	require.Nil(t, sm.gameServers[serverID])
}

func TestManagerShutdown(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	sm := createTestGameServerManager(t)
	require.NotEmpty(t, sm)

	serverID := uuid.New()
	config := &Config{}

	gs, err := sm.GetOrCreateGameServer(serverID, config)
	require.NoError(t, err)
	gs.shutdown()

	require.NotEmpty(t, sm.gameServers[serverID])

	sm.Shutdown()

	require.Nil(t, sm.gameServers[serverID])
}

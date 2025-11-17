package websocketserver

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func createTestGameServerManager(t *testing.T) *GameServerManager {
	gameServerManager := NewGameServerManager()
	require.NotEmpty(t, gameServerManager)
	require.Empty(t, gameServerManager.gameServers)

	return gameServerManager
}

func TestCreateGameServerManager(t *testing.T) {
	gameServerManager := createTestGameServerManager(t)
	require.NotEmpty(t, gameServerManager)
}

func TestGetOrCreateGameServer(t *testing.T) {
	sm := createTestGameServerManager(t)
	require.NotEmpty(t, sm)

	serverId := uuid.New()

	gsCall1 := sm.GetOrCreateGameServer(context.Background(), serverId)
	require.NotEmpty(t, gsCall1)

	gsCall2 := sm.GetOrCreateGameServer(context.Background(), serverId)
	require.NotEmpty(t, gsCall2)

	require.Equal(t, gsCall1, gsCall2)
}

func TestRemoveGameServerIfClosed(t *testing.T) {
	sm := createTestGameServerManager(t)
	require.NotEmpty(t, sm)

	serverId := uuid.New()

	gs := sm.GetOrCreateGameServer(context.Background(), serverId)
	gs.shutdown()

	require.NotEmpty(t, sm.gameServers[serverId])

	sm.RemoveGameServerIfClosed(serverId)

	require.Nil(t, sm.gameServers[serverId])
}

package gameserver

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func createTestClient(t *testing.T, isSpectating bool, startPumps bool) *Client {
	gameServer := createTestGameServer(t)
	require.NotEmpty(t, gameServer)

	client, err := NewClient(context.Background(), gameServer, nil, uuid.New(), isSpectating, startPumps)
	require.NoError(t, err)
	require.NotEmpty(t, client)
	require.NotEmpty(t, client.userId)
	require.NotEmpty(t, client.connId)
	require.NotEmpty(t, client.gameServer)
	require.Equal(t, isSpectating, client.isSpectating)

	return client
}

func TestCreateClient(t *testing.T) {
	client := createTestClient(t, false, true)

	require.NotEmpty(t, client)
}

package gameserver

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func createTestClient(t *testing.T, isSpectating bool) *Client {
	t.Helper()

	_, gameServer := createInitializedTestGameServer(t)

	client, err := NewClient(context.Background(), gameServer, nil, uuid.New(), isSpectating, WithoutPumps())
	require.NoError(t, err)
	require.NotEmpty(t, client)
	require.NotEmpty(t, client.userID)
	require.NotEmpty(t, client.connID)
	require.NotEmpty(t, client.gameServer)
	require.Equal(t, isSpectating, client.isSpectating)

	return client
}

func TestCreateClient(t *testing.T) {
	client := createTestClient(t, false)

	require.NotEmpty(t, client)
}

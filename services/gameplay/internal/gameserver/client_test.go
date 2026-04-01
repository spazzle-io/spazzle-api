package gameserver

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func createTestClient(t *testing.T, role Role) *Client {
	t.Helper()

	_, gameServer := createInitializedTestGameServer(t)

	client, err := NewClient(context.Background(), gameServer, nil, uuid.New(), role, WithoutPumps())
	require.NoError(t, err)
	require.NotEmpty(t, client)
	require.NotEmpty(t, client.userID)
	require.NotEmpty(t, client.connID)
	require.NotEmpty(t, client.gameServer)
	require.Equal(t, role, client.role.Load())
	require.Empty(t, client.timingUpdate)
	require.Equal(t, DefaultTiming, *client.currentTiming.Load())

	return client
}

func newStubClient(t *testing.T, gs *GameServer, userID uuid.UUID, role Role) *Client {
	t.Helper()

	client := &Client{
		userID:     userID,
		connID:     uuid.New(),
		gameServer: gs,
		send:       make(chan *OutgoingMessage, 8),
	}
	client.role.Store(role)

	return client
}

func TestCreateClient(t *testing.T) {
	client := createTestClient(t, Player)

	require.NotEmpty(t, client)
}

func TestUpdateTimingSendsOnChannel(t *testing.T) {
	client := createTestClient(t, Player)

	client.UpdateTiming(AggressiveTiming)

	select {
	case received := <-client.timingUpdate:
		require.Equal(t, AggressiveTiming, received)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected timing update on channel")
	}
}

func TestUpdateTimingDropsWhenChannelFull(t *testing.T) {
	client := createTestClient(t, Player)

	client.UpdateTiming(AggressiveTiming)
	// This should not block — it drops because the buffer is full
	client.UpdateTiming(DefaultTiming)

	select {
	case received := <-client.timingUpdate:
		require.Equal(t, AggressiveTiming, received)
	default:
		t.Fatal("expected AggressiveTiming timing update on channel")
	}

	select {
	case <-client.timingUpdate:
		t.Fatal("expected channel to be empty after draining")
	default:
	}
}

func TestUpdateTimingOnDisconnectedClient(t *testing.T) {
	client := createTestClient(t, Player)

	// Fill buffer to simulate a client whose writePump isn't draining
	client.timingUpdate <- AggressiveTiming

	require.NotPanics(t, func() {
		client.UpdateTiming(DefaultTiming)
	})
}

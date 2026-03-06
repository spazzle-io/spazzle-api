package gameserver

import (
	"context"
	"testing"
	"time"

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
	require.Empty(t, client.timingUpdate)
	require.Equal(t, DefaultTiming, *client.currentTiming.Load())

	return client
}

func TestCreateClient(t *testing.T) {
	client := createTestClient(t, false)

	require.NotEmpty(t, client)
}

func TestUpdateTimingSendsOnChannel(t *testing.T) {
	client := createTestClient(t, false)

	client.UpdateTiming(AggressiveTiming)

	select {
	case received := <-client.timingUpdate:
		require.Equal(t, AggressiveTiming, received)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected timing update on channel")
	}
}

func TestUpdateTimingDropsWhenChannelFull(t *testing.T) {
	client := createTestClient(t, false)

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
	client := createTestClient(t, false)

	// Fill buffer to simulate a client whose writePump isn't draining
	client.timingUpdate <- AggressiveTiming

	require.NotPanics(t, func() {
		client.UpdateTiming(DefaultTiming)
	})
}

package eventbus

import (
	"context"
	"testing"

	commonConfig "github.com/spazzle-io/spazzle-api/libs/common/config"

	"github.com/google/uuid"

	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/util"
	"github.com/stretchr/testify/require"
)

const testRedisConnURL = "redis://0.0.0.0:6379"

func newEventBus(t *testing.T) EventBus {
	ctx := context.Background()
	config := &util.Config{
		AppConfig: commonConfig.AppConfig{
			ServiceName: "test",
		},
		RedisConnURL: testRedisConnURL,
	}

	bus, err := New(ctx, config)
	require.NoError(t, err)
	require.NotNil(t, bus)

	t.Cleanup(func() {
		err = bus.Close()
		require.NoError(t, err)
	})

	return bus
}

func TestNew(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping event bus test in short mode")
	}

	bus := newEventBus(t)
	require.NotNil(t, bus)
}

func TestNewInvalidRedisConnURL(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping event bus test in short mode")
	}

	ctx := context.Background()
	config := &util.Config{
		AppConfig: commonConfig.AppConfig{
			ServiceName: "test",
		},
		RedisConnURL: "invalid-redis-conn-url",
	}

	bus, err := New(ctx, config)
	require.Error(t, err)
	require.Nil(t, bus)
}

func TestSession(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping event bus test in short mode")
	}

	bus := newEventBus(t)
	game := newGameIdentifier()

	session, err := bus.Session(game)
	require.NoError(t, err)
	require.NotNil(t, session)

	require.Equal(t, game, session.GameIdentifier())
}

func TestSession_ExistingSession(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping event bus test in short mode")
	}

	bus := newEventBus(t)
	game := newGameIdentifier()

	session1, err := bus.Session(game)
	require.NoError(t, err)

	session2, err := bus.Session(game)
	require.NoError(t, err)

	require.Same(t, session1, session2)
}

func TestSession_DifferentGames(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping event bus test in short mode")
	}

	bus := newEventBus(t)
	game1 := newGameIdentifier()
	game2 := newGameIdentifier()

	session1, err := bus.Session(game1)
	require.NoError(t, err)

	session2, err := bus.Session(game2)
	require.NoError(t, err)

	require.NotSame(t, session1, session2)
}

func TestSession_AfterClose(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping event bus test in short mode")
	}

	bus := newEventBus(t)

	err := bus.Close()
	require.NoError(t, err)

	session, err := bus.Session(newGameIdentifier())
	require.Error(t, err)
	require.ErrorIs(t, err, ErrClosedEventBus)
	require.Nil(t, session)
}

func TestReplay_AllMessages_NoClientFiltering(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping event bus test in short mode")
	}

	bus := newEventBus(t)
	game := newGameIdentifier()

	session, err := bus.Session(game)
	require.NoError(t, err)

	var publishedIDs []string
	for i := 0; i < 5; i++ {
		msg := PublishMessage{
			Type: "replay_event",
		}
		id, err := session.Publish(context.Background(), GameEventsStreamType, msg)
		require.NoError(t, err)
		publishedIDs = append(publishedIDs, id)
	}

	result, err := bus.Replay(context.Background(), uuid.Nil, game, GameEventsStreamType, ReplayVisibilityAll, "", StartFromBeginning().String(), 10)
	require.NoError(t, err)

	require.Len(t, result.Messages, 5)
	require.False(t, result.HasMore)
	require.Equal(t, publishedIDs[4], result.LastID)
}

func TestReplay_WithClientFiltering(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping event bus test in short mode")
	}

	bus := newEventBus(t)
	game := newGameIdentifier()

	session, err := bus.Session(game)
	require.NoError(t, err)

	clientID := uuid.New()

	var publishedIDs []string
	// publish some messages without a target client ID
	for i := 0; i < 2; i++ {
		msg := PublishMessage{
			Type: "replay_event",
		}
		id, err := session.Publish(context.Background(), GameEventsStreamType, msg)
		require.NoError(t, err)
		publishedIDs = append(publishedIDs, id)
	}
	// publish some messages with a different target client ID
	for i := 0; i < 4; i++ {
		msg := PublishMessage{
			Type:           "replay_event",
			TargetClientID: uuid.New(),
		}
		id, err := session.Publish(context.Background(), GameEventsStreamType, msg)
		require.NoError(t, err)
		publishedIDs = append(publishedIDs, id)
	}
	// publish some messages with this client's ID as the target
	for i := 0; i < 3; i++ {
		msg := PublishMessage{
			Type:           "replay_event",
			TargetClientID: clientID,
		}
		id, err := session.Publish(context.Background(), GameEventsStreamType, msg)
		require.NoError(t, err)
		publishedIDs = append(publishedIDs, id)
	}

	result, err := bus.Replay(context.Background(), clientID, game, GameEventsStreamType, ReplayVisibilityForClient, "", StartFromBeginning().String(), 10)
	require.NoError(t, err)

	require.Len(t, result.Messages, 5)
	require.False(t, result.HasMore)
	require.Equal(t, publishedIDs[8], result.LastID)
}

func TestReplay_BroadcastOnly(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping event bus test in short mode")
	}

	bus := newEventBus(t)
	game := newGameIdentifier()

	session, err := bus.Session(game)
	require.NoError(t, err)

	clientID := uuid.New()

	var publishedIDs []string
	// publish some messages without a target client ID
	for i := 0; i < 2; i++ {
		msg := PublishMessage{
			Type: "replay_event",
		}
		id, err := session.Publish(context.Background(), GameEventsStreamType, msg)
		require.NoError(t, err)
		publishedIDs = append(publishedIDs, id)
	}
	// publish some messages with a different target client ID
	for i := 0; i < 4; i++ {
		msg := PublishMessage{
			Type:           "replay_event",
			TargetClientID: uuid.New(),
		}
		id, err := session.Publish(context.Background(), GameEventsStreamType, msg)
		require.NoError(t, err)
		publishedIDs = append(publishedIDs, id)
	}
	// publish some messages with this client's ID as the target
	for i := 0; i < 3; i++ {
		msg := PublishMessage{
			Type:           "replay_event",
			TargetClientID: clientID,
		}
		id, err := session.Publish(context.Background(), GameEventsStreamType, msg)
		require.NoError(t, err)
		publishedIDs = append(publishedIDs, id)
	}

	result, err := bus.Replay(context.Background(), clientID, game, GameEventsStreamType, ReplayVisibilityBroadcastOnly, "", StartFromBeginning().String(), 10)
	require.NoError(t, err)

	require.Len(t, result.Messages, 2)
	require.False(t, result.HasMore)
	require.Equal(t, publishedIDs[8], result.LastID)
}

func TestReplay_WithLimit(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping event bus test in short mode")
	}

	bus := newEventBus(t)
	game := newGameIdentifier()

	session, err := bus.Session(game)
	require.NoError(t, err)

	for i := 0; i < 10; i++ {
		_, err := session.Publish(context.Background(), GameEventsStreamType, PublishMessage{Type: "event"})
		require.NoError(t, err)
	}

	result, err := bus.Replay(context.Background(), uuid.Nil, game, GameEventsStreamType, ReplayVisibilityAll, "", StartFromBeginning().String(), 3)
	require.NoError(t, err)

	require.Len(t, result.Messages, 3)
	require.True(t, result.HasMore)
	require.NotEmpty(t, result.LastID)
}

func TestReplay_ClientFiltering_InvalidClientId(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping event bus test in short mode")
	}

	bus := newEventBus(t)
	game := newGameIdentifier()

	result, err := bus.Replay(context.Background(), uuid.Nil, game, GameEventsStreamType, ReplayVisibilityForClient, "", StartFromBeginning().String(), 3)
	require.Error(t, err)

	require.Len(t, result.Messages, 0)
	require.False(t, result.HasMore)
	require.Empty(t, result.LastID)
}

func TestReplay_Pagination(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping event bus test in short mode")
	}

	bus := newEventBus(t)
	game := newGameIdentifier()

	session, err := bus.Session(game)
	require.NoError(t, err)

	for i := 0; i < 10; i++ {
		_, err := session.Publish(context.Background(), GameEventsStreamType, PublishMessage{Type: "event"})
		require.NoError(t, err)
	}

	// First page
	page1, err := bus.Replay(context.Background(), uuid.Nil, game, GameEventsStreamType, ReplayVisibilityAll, "", StartFromBeginning().String(), 4)
	require.NoError(t, err)
	require.Len(t, page1.Messages, 4)
	require.True(t, page1.HasMore)

	// Second page
	page2, err := bus.Replay(context.Background(), uuid.Nil, game, GameEventsStreamType, ReplayVisibilityAll, "", page1.LastID, 4)
	require.NoError(t, err)
	require.Len(t, page2.Messages, 4)
	require.True(t, page2.HasMore)

	// Third page
	page3, err := bus.Replay(context.Background(), uuid.Nil, game, GameEventsStreamType, ReplayVisibilityAll, "", page2.LastID, 4)
	require.NoError(t, err)
	require.Len(t, page3.Messages, 2)
	require.False(t, page3.HasMore)
}

func TestReplay_AfterClose(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping event bus test in short mode")
	}

	bus := newEventBus(t)

	err := bus.Close()
	require.NoError(t, err)

	game := newGameIdentifier()

	result, err := bus.Replay(context.Background(), uuid.New(), game, GameEventsStreamType, ReplayVisibilityAll, "", StartFromBeginning().String(), 4)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrClosedEventBus)

	require.Empty(t, result.Messages)
	require.False(t, result.HasMore)
	require.Empty(t, result.LastID)
}

func TestReplay_WithBeforeMarker(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping event bus test in short mode")
	}

	bus := newEventBus(t)
	game := newGameIdentifier()

	session, err := bus.Session(game)
	require.NoError(t, err)

	for i := 0; i < 3; i++ {
		_, err := session.Publish(context.Background(), GameEventsStreamType, PublishMessage{Type: "round1_event"})
		require.NoError(t, err)
	}

	endMarker := Marker{Type: MarkerRoundEnded, Round: 1}
	markerID, err := session.Publish(context.Background(), GameEventsStreamType, PublishMessage{
		Type:   "round_ended",
		Marker: endMarker,
	})
	require.NoError(t, err)

	for i := 0; i < 3; i++ {
		_, err := session.Publish(context.Background(), GameEventsStreamType, PublishMessage{Type: "round2_event"})
		require.NoError(t, err)
	}

	result, err := bus.Replay(context.Background(), uuid.Nil, game, GameEventsStreamType, ReplayVisibilityAll, markerID, "", 100)
	require.NoError(t, err)

	require.Len(t, result.Messages, 4)
	require.False(t, result.HasMore)

	for _, msg := range result.Messages {
		require.NotEqual(t, "round2_event", msg.Type)
	}

	err = bus.Cleanup(context.Background(), game)
	require.NoError(t, err)
}

func TestTrimStreamBefore(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping event bus test in short mode")
	}

	bus := newEventBus(t)
	game := newGameIdentifier()

	session, err := bus.Session(game)
	require.NoError(t, err)

	var ids []string
	for i := 0; i < 3; i++ {
		id, err := session.Publish(context.Background(), GameEventsStreamType, PublishMessage{Type: "event"})
		require.NoError(t, err)
		ids = append(ids, id)
	}

	for i := 0; i < 3; i++ {
		_, err := session.Publish(context.Background(), GameEventsStreamType, PublishMessage{Type: "event"})
		require.NoError(t, err)
	}

	err = bus.TrimStreamBefore(context.Background(), game, GameEventsStreamType, ids[2])
	require.NoError(t, err)

	result, err := bus.Replay(context.Background(), uuid.Nil, game, GameEventsStreamType, ReplayVisibilityAll, "", "", 100)
	require.NoError(t, err)
	require.Len(t, result.Messages, 4)
	require.Equal(t, ids[2], result.Messages[0].ID)

	err = bus.Cleanup(context.Background(), game)
	require.NoError(t, err)
}

func TestMarkerID(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping event bus test in short mode")
	}

	bus := newEventBus(t)
	game := newGameIdentifier()

	session, err := bus.Session(game)
	require.NoError(t, err)

	marker := Marker{Type: MarkerRoundEnded, Round: 1}

	msgID, err := session.Publish(context.Background(), GameEventsStreamType, PublishMessage{
		Type:   "event",
		Marker: marker,
	})
	require.NoError(t, err)

	markerID, err := bus.MarkerID(context.Background(), game, GameEventsStreamType, marker)
	require.NoError(t, err)
	require.Equal(t, msgID, markerID)

	err = bus.Cleanup(context.Background(), game)
	require.NoError(t, err)
}

func TestMarkerID_CacheMiss(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping event bus test in short mode")
	}

	bus := newEventBus(t)
	game := newGameIdentifier()

	marker := Marker{Type: MarkerRoundEnded, Round: 1}
	markerID, err := bus.MarkerID(context.Background(), game, GameEventsStreamType, marker)
	require.NoError(t, err)
	require.Empty(t, markerID)
}

func TestCleanup_RemovesAllRoundMarkers(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping event bus test in short mode")
	}

	bus := newEventBus(t)
	game := newGameIdentifier()

	session, err := bus.Session(game)
	require.NoError(t, err)

	for i := 1; i <= 3; i++ {
		startMarker := Marker{Type: MarkerRoundStarted, Round: uint8(i)}
		endMarker := Marker{Type: MarkerRoundEnded, Round: uint8(i)}

		_, err := session.Publish(context.Background(), GameEventsStreamType, PublishMessage{
			Type:   "round_started",
			Marker: startMarker,
		})
		require.NoError(t, err)

		_, err = session.Publish(context.Background(), GameEventsStreamType, PublishMessage{
			Type:   "round_ended",
			Marker: endMarker,
		})
		require.NoError(t, err)
	}

	err = bus.Cleanup(context.Background(), game)
	require.NoError(t, err)

	for i := 1; i <= 3; i++ {
		startMarker := Marker{Type: MarkerRoundStarted, Round: uint8(i)}
		endMarker := Marker{Type: MarkerRoundEnded, Round: uint8(i)}

		id, err := bus.MarkerID(context.Background(), game, GameEventsStreamType, startMarker)
		require.NoError(t, err)
		require.Empty(t, id)

		id, err = bus.MarkerID(context.Background(), game, GameEventsStreamType, endMarker)
		require.NoError(t, err)
		require.Empty(t, id)
	}
}

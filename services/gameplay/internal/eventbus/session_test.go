package eventbus

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func newGameIdentifier() GameIdentifier {
	return GameIdentifier{
		GameServerID: uuid.New(),
		GameID:       uuid.New(),
	}
}

func TestPublish(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping event bus test in short mode")
	}

	bus := newEventBus(t)
	game := newGameIdentifier()

	session, err := bus.Session(game)
	require.NoError(t, err)

	msg := PublishMessage{
		Type:    "test_event",
		Payload: json.RawMessage(`{"key":"value"}`),
	}

	messageID, err := session.Publish(context.Background(), GameEventsStreamType, msg)
	require.NoError(t, err)
	require.NotEmpty(t, messageID)
}

func TestPublish_WithMarker(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping event bus test in short mode")
	}

	bus := newEventBus(t)
	game := newGameIdentifier()

	session, err := bus.Session(game)
	require.NoError(t, err)

	msg := PublishMessage{
		Type:    "test_event",
		Payload: json.RawMessage(`{"key":"value"}`),
		Marker:  MarkerRoundEnded,
	}

	messageID, err := session.Publish(context.Background(), GameEventsStreamType, msg)
	require.NoError(t, err)
	require.NotEmpty(t, messageID)

	markerID, err := bus.MarkerID(context.Background(), game, GameEventsStreamType, MarkerRoundEnded)
	require.NoError(t, err)
	require.Equal(t, messageID, markerID)

	err = bus.Cleanup(context.Background(), game)
	require.NoError(t, err)
}

func TestPublish_AfterClose(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping event bus test in short mode")
	}

	bus := newEventBus(t)
	game := newGameIdentifier()

	session, err := bus.Session(game)
	require.NoError(t, err)

	session.Close()

	msg := PublishMessage{
		Type:    "test_event",
		Payload: json.RawMessage(`{"key":"value"}`),
	}

	messageID, err := session.Publish(context.Background(), GameEventsStreamType, msg)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrSessionClosed)
	require.Empty(t, messageID)
}

func TestSubscribe(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping event bus test in short mode")
	}

	bus := newEventBus(t)
	game := newGameIdentifier()

	session, err := bus.Session(game)
	require.NoError(t, err)

	var received []Message
	var mu sync.Mutex

	handler := func(ctx context.Context, msg Message) {
		mu.Lock()
		received = append(received, msg)
		mu.Unlock()
	}

	err = session.Subscribe(context.Background(), GameEventsStreamType, StartFromNow(), handler)
	require.NoError(t, err)

	msg := PublishMessage{
		Type:    "test_event",
		Payload: json.RawMessage(`{"test":"data"}`),
	}
	_, err = session.Publish(context.Background(), GameEventsStreamType, msg)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(received) == 1
	}, 2*time.Second, 50*time.Millisecond)

	mu.Lock()
	require.Equal(t, "test_event", received[0].Type)
	mu.Unlock()
}

func TestSubscribe_MultipleMessages(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping event bus test in short mode")
	}

	bus := newEventBus(t)
	game := newGameIdentifier()

	session, err := bus.Session(game)
	require.NoError(t, err)

	var received []Message
	var mu sync.Mutex

	handler := func(ctx context.Context, msg Message) {
		mu.Lock()
		received = append(received, msg)
		mu.Unlock()
	}

	err = session.Subscribe(context.Background(), GameEventsStreamType, StartFromNow(), handler)
	require.NoError(t, err)

	for i := 0; i < 5; i++ {
		msg := PublishMessage{
			Type:    "test_event",
			Payload: json.RawMessage(`{}`),
		}
		_, err = session.Publish(context.Background(), GameEventsStreamType, msg)
		require.NoError(t, err)
	}

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(received) == 5
	}, 2*time.Second, 50*time.Millisecond)
}

func TestSubscribe_FromBeginning(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping event bus test in short mode")
	}

	bus := newEventBus(t)
	game := newGameIdentifier()

	session, err := bus.Session(game)
	require.NoError(t, err)

	for i := 0; i < 3; i++ {
		msg := PublishMessage{
			Type:    "pre_subscribe_event",
			Payload: json.RawMessage(`{}`),
		}
		_, err = session.Publish(context.Background(), GameEventsStreamType, msg)
		require.NoError(t, err)
	}

	var received []Message
	var mu sync.Mutex

	handler := func(ctx context.Context, msg Message) {
		mu.Lock()
		received = append(received, msg)
		mu.Unlock()
	}

	err = session.Subscribe(context.Background(), GameEventsStreamType, StartFromBeginning(), handler)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(received) == 3
	}, 2*time.Second, 50*time.Millisecond)
}

func TestSubscribe_FromNow_MidStream(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping event bus test in short mode")
	}

	bus := newEventBus(t)
	game := newGameIdentifier()

	session, err := bus.Session(game)
	require.NoError(t, err)

	msg := PublishMessage{
		Type:    "pre_subscribe_event",
		Payload: json.RawMessage(`{}`),
	}
	_, err = session.Publish(context.Background(), GameEventsStreamType, msg)
	require.NoError(t, err)

	var received []string
	var mu sync.Mutex

	handler := func(ctx context.Context, msg Message) {
		mu.Lock()
		received = append(received, msg.Type)
		mu.Unlock()
	}

	err = session.Subscribe(context.Background(), GameEventsStreamType, StartFromNow(), handler)
	require.NoError(t, err)

	for i := 0; i < 3; i++ {
		msg := PublishMessage{
			Type:    "post_subscribe_event",
			Payload: json.RawMessage(`{}`),
		}
		_, err = session.Publish(context.Background(), GameEventsStreamType, msg)
		require.NoError(t, err)
	}

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(received) == 3
	}, 2*time.Second, 50*time.Millisecond)

	require.Equal(t, []string{"post_subscribe_event", "post_subscribe_event", "post_subscribe_event"}, received)
}

func TestSubscribe_NoOpIfAlreadySubscribed(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping event bus test in short mode")
	}

	bus := newEventBus(t)
	game := newGameIdentifier()

	session, err := bus.Session(game)
	require.NoError(t, err)

	var handler1Called, handler2Called bool
	var mu sync.Mutex

	handler1 := func(ctx context.Context, msg Message) {
		mu.Lock()
		handler1Called = true
		mu.Unlock()
	}

	handler2 := func(ctx context.Context, msg Message) {
		mu.Lock()
		handler2Called = true
		mu.Unlock()
	}

	err = session.Subscribe(context.Background(), GameEventsStreamType, StartFromNow(), handler1)
	require.NoError(t, err)

	err = session.Subscribe(context.Background(), GameEventsStreamType, StartFromNow(), handler2)
	require.NoError(t, err)

	_, err = session.Publish(context.Background(), GameEventsStreamType, PublishMessage{Type: "test"})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return handler1Called
	}, 2*time.Second, 50*time.Millisecond)

	mu.Lock()
	require.True(t, handler1Called)
	require.False(t, handler2Called)
	mu.Unlock()
}

func TestSubscribe_AfterClose(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping event bus test in short mode")
	}

	bus := newEventBus(t)
	game := newGameIdentifier()

	session, err := bus.Session(game)
	require.NoError(t, err)

	session.Close()

	handler := func(ctx context.Context, msg Message) {}

	err = session.Subscribe(context.Background(), GameEventsStreamType, StartFromNow(), handler)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrSessionClosed)
}

func TestUnsubscribe(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping event bus test in short mode")
	}

	bus := newEventBus(t)
	game := newGameIdentifier()

	session, err := bus.Session(game)
	require.NoError(t, err)

	var received []Message
	var mu sync.Mutex

	handler := func(ctx context.Context, msg Message) {
		mu.Lock()
		received = append(received, msg)
		mu.Unlock()
	}

	err = session.Subscribe(context.Background(), GameEventsStreamType, StartFromNow(), handler)
	require.NoError(t, err)

	_, err = session.Publish(context.Background(), GameEventsStreamType, PublishMessage{Type: "event1"})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(received) == 1
	}, 2*time.Second, 50*time.Millisecond)

	session.Unsubscribe(GameEventsStreamType)

	_, err = session.Publish(context.Background(), GameEventsStreamType, PublishMessage{Type: "event2"})
	require.NoError(t, err)

	time.Sleep(200 * time.Millisecond)
	mu.Lock()
	require.Len(t, received, 1)
	mu.Unlock()
}

func TestUnsubscribe_NoOpIfNotSubscribed(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping event bus test in short mode")
	}

	bus := newEventBus(t)
	game := newGameIdentifier()

	session, err := bus.Session(game)
	require.NoError(t, err)

	session.Unsubscribe(GameEventsStreamType) // does not panic or error
}

func TestUnsubscribe_AfterClose(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping event bus test in short mode")
	}

	bus := newEventBus(t)
	game := newGameIdentifier()

	session, err := bus.Session(game)
	require.NoError(t, err)

	session.Close()

	session.Unsubscribe(GameEventsStreamType) // does not panic or error
}

func TestClose(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping event bus test in short mode")
	}

	bus := newEventBus(t)
	game := newGameIdentifier()

	session, err := bus.Session(game)
	require.NoError(t, err)

	err = session.Subscribe(context.Background(), GameEventsStreamType, StartFromNow(), func(ctx context.Context, msg Message) {})
	require.NoError(t, err)

	session.Close()

	session.Close() // does not panic or error if Close is called multiple times
}

func TestClose_StopsSubscription(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping event bus test in short mode")
	}

	bus := newEventBus(t)
	game := newGameIdentifier()

	session, err := bus.Session(game)
	require.NoError(t, err)

	var received []Message
	var mu sync.Mutex

	handler := func(ctx context.Context, msg Message) {
		mu.Lock()
		received = append(received, msg)
		mu.Unlock()
	}

	err = session.Subscribe(context.Background(), GameEventsStreamType, StartFromNow(), handler)
	require.NoError(t, err)

	session.Close()

	session2, err := bus.Session(game)
	require.NoError(t, err)
	_, err = session2.Publish(context.Background(), GameEventsStreamType, PublishMessage{Type: "after_close"})
	require.NoError(t, err)

	time.Sleep(500 * time.Millisecond)
	mu.Lock()
	require.Empty(t, received)
	mu.Unlock()
}

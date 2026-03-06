package eventbus

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestMultipleGamesOnSameBus(t *testing.T) {
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

	var received1, received2 []Message
	var mu sync.Mutex

	handler1 := func(ctx context.Context, msg Message) {
		mu.Lock()
		received1 = append(received1, msg)
		mu.Unlock()
	}

	handler2 := func(ctx context.Context, msg Message) {
		mu.Lock()
		received2 = append(received2, msg)
		mu.Unlock()
	}

	err = session1.Subscribe(context.Background(), GameEventsStreamType, StartFromNow(), handler1)
	require.NoError(t, err)

	err = session2.Subscribe(context.Background(), GameEventsStreamType, StartFromNow(), handler2)
	require.NoError(t, err)

	_, err = session1.Publish(context.Background(), GameEventsStreamType, PublishMessage{Type: "game1_event"})
	require.NoError(t, err)

	_, err = session2.Publish(context.Background(), GameEventsStreamType, PublishMessage{Type: "game2_event"})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(received1) == 1 && len(received2) == 1
	}, 2*time.Second, 50*time.Millisecond)

	mu.Lock()
	require.Equal(t, "game1_event", received1[0].Type)
	require.Equal(t, "game2_event", received2[0].Type)
	mu.Unlock()
}

func TestSubscribeToBothStreamTypes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping event bus test in short mode")
	}

	bus := newEventBus(t)
	game := newGameIdentifier()

	session, err := bus.Session(game)
	require.NoError(t, err)

	var gameEventsReceived, drawingUpdatesReceived []Message
	var mu sync.Mutex

	gameEventsHandler := func(ctx context.Context, msg Message) {
		mu.Lock()
		gameEventsReceived = append(gameEventsReceived, msg)
		mu.Unlock()
	}

	drawingUpdatesHandler := func(ctx context.Context, msg Message) {
		mu.Lock()
		drawingUpdatesReceived = append(drawingUpdatesReceived, msg)
		mu.Unlock()
	}

	err = session.Subscribe(context.Background(), GameEventsStreamType, StartFromNow(), gameEventsHandler)
	require.NoError(t, err)

	err = session.Subscribe(context.Background(), DrawingUpdatesStreamType, StartFromNow(), drawingUpdatesHandler)
	require.NoError(t, err)

	_, err = session.Publish(context.Background(), GameEventsStreamType, PublishMessage{Type: "game_event"})
	require.NoError(t, err)

	_, err = session.Publish(context.Background(), DrawingUpdatesStreamType, PublishMessage{Type: "drawing_update"})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(gameEventsReceived) == 1 && len(drawingUpdatesReceived) == 1
	}, 2*time.Second, 50*time.Millisecond)

	mu.Lock()
	require.Equal(t, "game_event", gameEventsReceived[0].Type)
	require.Equal(t, "drawing_update", drawingUpdatesReceived[0].Type)
	mu.Unlock()
}

func TestMessageOrderPreserved(t *testing.T) {
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

	for i := 0; i < 10; i++ {
		_, err = session.Publish(context.Background(), GameEventsStreamType, PublishMessage{
			Type:    "ordered_event",
			Payload: json.RawMessage(`{"order":` + string(rune('0'+i)) + `}`),
		})
		require.NoError(t, err)
	}

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(received) == 10
	}, 2*time.Second, 50*time.Millisecond)

	mu.Lock()
	for i := 1; i < len(received); i++ {
		require.True(t, received[i].Timestamp.After(received[i-1].Timestamp) ||
			received[i].Timestamp.Equal(received[i-1].Timestamp))
	}
	mu.Unlock()
}

func TestMultiplexerHandlesUnsubscribeMidStream(t *testing.T) {
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

	for i := 0; i < 3; i++ {
		_, err = session.Publish(context.Background(), GameEventsStreamType, PublishMessage{Type: "before_unsub"})
		require.NoError(t, err)
	}

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(received) == 3
	}, 2*time.Second, 50*time.Millisecond)

	session.Unsubscribe(GameEventsStreamType)

	for i := 0; i < 3; i++ {
		_, err = session.Publish(context.Background(), GameEventsStreamType, PublishMessage{Type: "after_unsub"})
		require.NoError(t, err)
	}

	time.Sleep(200 * time.Millisecond)
	mu.Lock()
	require.Len(t, received, 3)
	mu.Unlock()
}

func TestMultiplexerContextCancellation(t *testing.T) {
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

	ctx, cancel := context.WithCancel(context.Background())
	err = session.Subscribe(ctx, GameEventsStreamType, StartFromNow(), handler)
	require.NoError(t, err)

	_, err = session.Publish(context.Background(), GameEventsStreamType, PublishMessage{Type: "before_cancel"})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(received) == 1
	}, 2*time.Second, 50*time.Millisecond)

	cancel()

	time.Sleep(200 * time.Millisecond)

	_, err = session.Publish(context.Background(), GameEventsStreamType, PublishMessage{Type: "after_cancel"})
	require.NoError(t, err)

	time.Sleep(200 * time.Millisecond)
	mu.Lock()
	require.Len(t, received, 1)
	mu.Unlock()
}

func TestHighVolumeMessages(t *testing.T) {
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

	err = session.Subscribe(context.Background(), DrawingUpdatesStreamType, StartFromNow(), handler)
	require.NoError(t, err)

	messageCount := 100
	for i := 0; i < messageCount; i++ {
		_, err = session.Publish(context.Background(), DrawingUpdatesStreamType, PublishMessage{
			Type:    "stroke",
			Payload: json.RawMessage(`{"x":100,"y":200}`),
		})
		require.NoError(t, err)
	}

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(received) == messageCount
	}, 5*time.Second, 50*time.Millisecond)
}

func TestMultiplexerCalculateBackoff(t *testing.T) {
	retryBaseDelay := 100 * time.Millisecond
	retryMaxDelay := 800 * time.Millisecond
	maxRetries := 5

	multiplexer := multiplexer{
		config: Config{
			RetryBaseDelay: retryBaseDelay,
			RetryMaxDelay:  retryMaxDelay,
			MaxRetries:     maxRetries,
		},
	}

	testCases := []struct {
		name              string
		consecutiveErrors int
		expectedBackoff   time.Duration
	}{
		{
			name:              "zero errors",
			consecutiveErrors: 0,
			expectedBackoff:   retryBaseDelay,
		},
		{
			name:              "one error",
			consecutiveErrors: 1,
			expectedBackoff:   retryBaseDelay,
		},
		{
			name:              "two errors",
			consecutiveErrors: 2,
			expectedBackoff:   200 * time.Millisecond,
		},
		{
			name:              "three errors",
			consecutiveErrors: 3,
			expectedBackoff:   400 * time.Millisecond,
		},
		{
			name:              "four errors",
			consecutiveErrors: 4,
			expectedBackoff:   800 * time.Millisecond,
		},
		{
			name:              "five errors",
			consecutiveErrors: 4,
			expectedBackoff:   800 * time.Millisecond,
		},
		{
			name:              "beyond max retries",
			consecutiveErrors: 40,
			expectedBackoff:   800 * time.Millisecond,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			backoff := multiplexer.calculateBackoff(tc.consecutiveErrors)
			require.Equal(t, tc.expectedBackoff, backoff)
		})
	}
}

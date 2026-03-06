package eventbus

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/util"
)

var ErrClosedEventBus = errors.New("event bus is closed")

type redisEventBus struct {
	client        *redis.Client
	busConfig     Config
	serviceConfig util.Config

	gameEventsMultiplexer     *multiplexer
	drawingUpdatesMultiplexer *multiplexer

	mu       sync.RWMutex
	sessions map[GameIdentifier]*redisSession
	closed   bool
}

func New(ctx context.Context, config util.Config, opts ...Option) (EventBus, error) {
	redisOpts, err := redis.ParseURL(config.RedisConnURL)
	if err != nil {
		return nil, fmt.Errorf("invalid redis connection URL: %w", err)
	}

	client := redis.NewClient(redisOpts)

	c := Config{}
	for _, opt := range opts {
		opt(&c)
	}
	c.applyDefaults()

	bus := &redisEventBus{
		client:        client,
		busConfig:     c,
		serviceConfig: config,
		sessions:      make(map[GameIdentifier]*redisSession),
	}

	bus.gameEventsMultiplexer = newMultiplexer(ctx, client, c, GameEventsStreamType)
	bus.drawingUpdatesMultiplexer = newMultiplexer(ctx, client, c, DrawingUpdatesStreamType)

	log.Info().Msg("created event bus")

	return bus, nil
}

func (b *redisEventBus) Session(game GameIdentifier) (Session, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return nil, ErrClosedEventBus
	}

	if session, ok := b.sessions[game]; ok {
		return session, nil
	}

	session := newRedisSession(b, game)
	b.sessions[game] = session
	session.getLogger().Info().Msg("created new event bus session")

	return session, nil
}

func (b *redisEventBus) removeSession(game GameIdentifier) {
	b.mu.Lock()
	defer b.mu.Unlock()

	delete(b.sessions, game)
}

func (b *redisEventBus) multiplexerFor(streamType StreamType) *multiplexer {
	switch streamType {
	case DrawingUpdatesStreamType:
		return b.drawingUpdatesMultiplexer
	default:
		return b.gameEventsMultiplexer
	}
}

func (b *redisEventBus) Replay(ctx context.Context, game GameIdentifier, streamType StreamType, after string, limit int) (ReplayResult, error) {
	b.mu.RLock()
	if b.closed {
		b.mu.RUnlock()
		return ReplayResult{}, ErrClosedEventBus
	}
	b.mu.RUnlock()

	sk := streamKey(b.serviceConfig, streamType, game)

	start := after
	if start == "" || start == "0" {
		start = "-" // minimum ID
	} else {
		start = "(" + start // exclusive start
	}

	count := int64(limit + 1)
	msgs, err := b.client.XRangeN(ctx, sk, start, "+", count).Result()
	if err != nil {
		return ReplayResult{}, fmt.Errorf("event bus failed to replay messages: %w", err)
	}

	hasMore := len(msgs) > limit
	if hasMore {
		msgs = msgs[:limit]
	}

	messages := make([]Message, 0, len(msgs))
	var lastID string
	for _, redisMsg := range msgs {
		msg, err := decodeMessage(redisMsg.ID, redisMsg.Values)
		if err != nil {
			log.Error().
				Err(err).
				Str("id", redisMsg.ID).
				Str("stream_key", sk).
				Any("msg", redisMsg.Values).
				Msg("failed to decode redis event message")
			continue
		}

		messages = append(messages, msg)
		lastID = redisMsg.ID
	}

	return ReplayResult{
		Messages: messages,
		HasMore:  hasMore,
		LastID:   lastID,
	}, nil
}

func (b *redisEventBus) Close() error {
	b.mu.Lock()

	if b.closed {
		b.mu.Unlock()
		log.Info().Msg("event bus is already closed")
		return nil
	}
	b.closed = true

	sessions := make([]*redisSession, 0, len(b.sessions))
	for _, s := range b.sessions {
		sessions = append(sessions, s)
	}
	b.sessions = nil
	b.mu.Unlock()

	for _, s := range sessions {
		s.Close()
	}

	b.gameEventsMultiplexer.stop()
	b.drawingUpdatesMultiplexer.stop()

	err := b.client.Close()
	if err != nil {
		return fmt.Errorf("could not close event bus: %w", err)
	}

	log.Info().Msg("closed event bus")

	return nil
}

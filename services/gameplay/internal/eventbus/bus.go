package eventbus

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/google/uuid"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/util"
)

var ErrClosedEventBus = errors.New("event bus is closed")

type redisEventBus struct {
	client        *redis.Client
	busConfig     Config
	serviceConfig *util.Config

	gameEventsMultiplexer     *multiplexer
	drawingUpdatesMultiplexer *multiplexer

	mu       sync.RWMutex
	sessions map[GameIdentifier]*redisSession
	closed   bool
}

func New(ctx context.Context, config *util.Config, opts ...Option) (EventBus, error) {
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

func shouldIncludeReplayMsg(msg Message, visibility ReplayVisibility, clientID uuid.UUID) bool {
	switch visibility {
	case ReplayVisibilityAll:
		return true

	case ReplayVisibilityBroadcastOnly:
		return msg.TargetClientID == uuid.Nil

	case ReplayVisibilityForClient:
		return msg.TargetClientID == uuid.Nil || msg.TargetClientID == clientID

	default:
		return false
	}
}

func (b *redisEventBus) Replay(
	ctx context.Context,
	clientID uuid.UUID,
	game GameIdentifier,
	streamType StreamType,
	visibility ReplayVisibility,
	after string,
	limit int,
) (ReplayResult, error) {
	if visibility == ReplayVisibilityForClient && clientID == uuid.Nil {
		return ReplayResult{}, errors.New("clientID required for ReplayVisibilityForClient")
	}

	b.mu.RLock()
	if b.closed {
		b.mu.RUnlock()
		return ReplayResult{}, ErrClosedEventBus
	}
	b.mu.RUnlock()

	sk := streamKey(b.serviceConfig, streamType, game)

	cursor := after
	if cursor == "" || cursor == "0" {
		cursor = "-" // minimum ID
	} else {
		cursor = "(" + cursor // exclusive start
	}

	batchSize := int64(limit)
	messages := make([]Message, 0, limit)
	var lastID string

	for {
		msgs, err := b.client.XRangeN(ctx, sk, cursor, "+", batchSize).Result()
		if err != nil {
			return ReplayResult{}, fmt.Errorf("event bus failed to replay messages: %w", err)
		}

		if len(msgs) == 0 {
			break
		}

		for _, redisMsg := range msgs {
			lastID = redisMsg.ID

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

			if !shouldIncludeReplayMsg(msg, visibility, clientID) {
				continue
			}

			messages = append(messages, msg)

			if len(messages) >= limit {
				return ReplayResult{
					Messages: messages,
					HasMore:  true,
					LastID:   lastID,
				}, nil
			}
		}

		cursor = "(" + msgs[len(msgs)-1].ID

		if int64(len(msgs)) < batchSize {
			break
		}
	}

	return ReplayResult{
		Messages: messages,
		HasMore:  false,
		LastID:   lastID,
	}, nil
}

func (b *redisEventBus) MarkerID(
	ctx context.Context,
	game GameIdentifier,
	streamType StreamType,
	marker Marker,
) (string, error) {
	b.mu.RLock()
	if b.closed {
		b.mu.RUnlock()
		return "", ErrClosedEventBus
	}
	b.mu.RUnlock()

	key := markerKey(b.serviceConfig, streamType, game, marker)
	id, err := b.client.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("failed to get marker: %w", err)
	}

	return id, nil
}

func (b *redisEventBus) deleteStreams(ctx context.Context, game GameIdentifier) error {
	keys := make([]string, 0, len(AllStreamTypes))
	for _, st := range AllStreamTypes {
		keys = append(keys, streamKey(b.serviceConfig, st, game))
	}

	if err := b.client.Del(ctx, keys...).Err(); err != nil {
		return fmt.Errorf("failed to delete streams: %w", err)
	}

	return nil
}

func (b *redisEventBus) deleteMarkers(ctx context.Context, game GameIdentifier) error {
	keys := make([]string, 0, len(AllStreamTypes)*len(AllMarkers))
	for _, st := range AllStreamTypes {
		for _, m := range AllMarkers {
			keys = append(keys, markerKey(b.serviceConfig, st, game, m))
		}
	}

	if err := b.client.Del(ctx, keys...).Err(); err != nil {
		return fmt.Errorf("failed to delete markers: %w", err)
	}

	return nil
}

func (b *redisEventBus) Cleanup(ctx context.Context, game GameIdentifier) error {
	b.mu.RLock()
	if b.closed {
		b.mu.RUnlock()
		return ErrClosedEventBus
	}
	b.mu.RUnlock()

	if err := b.deleteStreams(ctx, game); err != nil {
		return err
	}

	return b.deleteMarkers(ctx, game)
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

package eventbus

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var (
	ErrSessionClosed = errors.New("redis event bus session is closed")
	ErrPublishFailed = errors.New("failed to publish message to event bus")
)

type redisSession struct {
	bus  *redisEventBus
	game GameIdentifier

	mu     sync.Mutex
	closed bool

	subscriptions map[StreamType]context.CancelFunc
}

func newRedisSession(bus *redisEventBus, game GameIdentifier) *redisSession {
	return &redisSession{
		bus:           bus,
		game:          game,
		subscriptions: make(map[StreamType]context.CancelFunc),
	}
}

func (s *redisSession) getLogger() *zerolog.Logger {
	logger := log.With().
		Str("game_server_id", s.game.GameServerID.String()).
		Str("game_id", s.game.GameID.String()).
		Logger()

	return &logger
}

func (s *redisSession) GameIdentifier() GameIdentifier {
	return s.game
}

func (s *redisSession) Subscribe(ctx context.Context, streamType StreamType, startFrom StartPosition, handler MessageHandler) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrSessionClosed
	}

	if _, ok := s.subscriptions[streamType]; ok {
		return nil
	}

	subCtx, subCancel := context.WithCancel(ctx)
	s.subscriptions[streamType] = subCancel

	streamKey := streamKey(s.bus.serviceConfig, streamType, s.game)
	s.bus.multiplexerFor(streamType).subscribe(subCtx, streamKey, startFrom, handler)

	s.getLogger().Info().Str("stream_type", string(streamType)).Msg("subscribed to event bus stream")

	return nil
}

func (s *redisSession) Unsubscribe(streamType StreamType) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return
	}

	if cancel, ok := s.subscriptions[streamType]; ok {
		cancel()
		delete(s.subscriptions, streamType)
		streamKey := streamKey(s.bus.serviceConfig, streamType, s.game)
		s.bus.multiplexerFor(streamType).unsubscribe(streamKey)
		s.getLogger().Info().Str("stream_type", string(streamType)).Msg("unsubscribed from event bus stream")
	}
}

func (s *redisSession) Publish(ctx context.Context, streamType StreamType, msg Message) (string, error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return "", ErrSessionClosed
	}
	s.mu.Unlock()

	fields, err := encodeMessage(msg)
	if err != nil {
		return "", err
	}

	sk := streamKey(s.bus.serviceConfig, streamType, s.game)
	id, err := s.bus.client.XAdd(ctx, &redis.XAddArgs{
		Stream: sk,
		Values: fields,
	}).Result()
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrPublishFailed, err)
	}

	return id, nil
}

func (s *redisSession) PublishTargeted(ctx context.Context, streamType StreamType, clientID uuid.UUID, correlationID uuid.UUID, msg Message) (string, error) {
	msg.TargetClientID = &clientID
	msg.CorrelationID = &correlationID

	return s.Publish(ctx, streamType, msg)
}

func (s *redisSession) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return
	}
	s.closed = true

	for streamType, cancel := range s.subscriptions {
		cancel()
		sk := streamKey(s.bus.serviceConfig, streamType, s.game)
		s.bus.multiplexerFor(streamType).unsubscribe(sk)
	}
	s.subscriptions = nil

	s.bus.removeSession(s.game)
	s.getLogger().Info().Msg("event bus session closed")
}

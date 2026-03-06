package eventbus

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

const idlePollInterval = 500 * time.Millisecond

type subscription struct {
	streamKey string
	lastID    string
	handler   MessageHandler
	ctx       context.Context
	cancel    context.CancelFunc
}

type multiplexer struct {
	client     *redis.Client
	config     Config
	streamType StreamType

	mu            sync.RWMutex
	subscriptions map[string]*subscription

	ctx    context.Context
	cancel context.CancelFunc

	xReadMu     sync.Mutex
	xReadCancel context.CancelFunc

	wg sync.WaitGroup
}

func newMultiplexer(ctx context.Context, client *redis.Client, config Config, streamType StreamType) *multiplexer {
	ctx, cancel := context.WithCancel(ctx)

	m := &multiplexer{
		client:        client,
		config:        config,
		streamType:    streamType,
		subscriptions: make(map[string]*subscription),
		ctx:           ctx,
		cancel:        cancel,
	}

	m.wg.Add(1)
	go m.run()

	log.Info().Str("stream_type", string(streamType)).Msg("created event bus multiplexer")

	return m
}

func (m *multiplexer) subscribe(ctx context.Context, streamKey string, startFrom StartPosition, handler MessageHandler) {
	subCtx, subCancel := context.WithCancel(ctx)

	lastID := startFrom.String()
	if lastID == StartFromNow().String() {
		lastID = m.resolveCurrentStreamID(streamKey)
	}

	sub := &subscription{
		streamKey: streamKey,
		lastID:    lastID,
		handler:   handler,
		ctx:       subCtx,
		cancel:    subCancel,
	}

	m.mu.Lock()
	m.subscriptions[streamKey] = sub
	m.mu.Unlock()

	m.interruptXRead()
}

func (m *multiplexer) resolveCurrentStreamID(streamKey string) string {
	msgs, err := m.client.XRevRangeN(context.Background(), streamKey, "+", "-", 1).Result()
	if err != nil || len(msgs) == 0 {
		return "0"
	}

	return msgs[0].ID
}

func (m *multiplexer) unsubscribe(streamKey string) {
	m.mu.Lock()
	if sub, ok := m.subscriptions[streamKey]; ok {
		sub.cancel()
		delete(m.subscriptions, streamKey)
	}
	m.mu.Unlock()

	m.interruptXRead()
}

func (m *multiplexer) interruptXRead() {
	m.xReadMu.Lock()
	if m.xReadCancel != nil {
		m.xReadCancel()
	}
	m.xReadMu.Unlock()
}

func (m *multiplexer) buildXReadArgs() (streams []string, lastIDs []string, subs map[string]*subscription) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	subs = make(map[string]*subscription, len(m.subscriptions))
	streams = make([]string, 0, len(m.subscriptions))
	lastIDs = make([]string, 0, len(m.subscriptions))

	for streamKey, sub := range m.subscriptions {
		if sub.ctx.Err() != nil {
			continue
		}

		subs[streamKey] = sub
		streams = append(streams, streamKey)
		lastIDs = append(lastIDs, sub.lastID)
	}

	return
}

func (m *multiplexer) routeMessages(results []redis.XStream, subs map[string]*subscription) {
	for _, stream := range results {
		sub, ok := subs[stream.Stream]
		if !ok {
			continue
		}

		if sub.ctx.Err() != nil {
			continue
		}

		for _, redisMsg := range stream.Messages {
			msg, err := decodeMessage(redisMsg.ID, redisMsg.Values)
			if err != nil {
				log.Error().
					Err(err).
					Str("id", redisMsg.ID).
					Str("stream_key", stream.Stream).
					Any("msg", redisMsg.Values).
					Msg("failed to decode redis event message")
				m.updateLastID(stream.Stream, redisMsg.ID)
				continue
			}

			sub.handler(sub.ctx, msg)
			m.updateLastID(stream.Stream, redisMsg.ID)
		}
	}
}

func (m *multiplexer) updateLastID(streamKey string, id string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if sub, ok := m.subscriptions[streamKey]; ok {
		sub.lastID = id
	}
}

func (m *multiplexer) run() {
	defer m.wg.Done()

	consecutiveErrors := 0

	for {
		if m.ctx.Err() != nil {
			return
		}

		streams, lastIDs, subs := m.buildXReadArgs()

		if len(streams) == 0 {
			select {
			case <-m.ctx.Done():
				return
			case <-time.After(idlePollInterval):
				continue
			}
		}

		xReadCtx, xReadCancel := context.WithCancel(m.ctx)
		m.xReadMu.Lock()
		m.xReadCancel = xReadCancel
		m.xReadMu.Unlock()

		args := &redis.XReadArgs{
			Streams: append(streams, lastIDs...),
			Block:   m.config.ReadBlockDuration,
		}
		results, err := m.client.XRead(xReadCtx, args).Result()

		m.xReadMu.Lock()
		m.xReadCancel = nil
		m.xReadMu.Unlock()

		if err != nil {
			if errors.Is(err, context.Canceled) {
				consecutiveErrors = 0
				continue
			}
			if errors.Is(err, redis.Nil) {
				consecutiveErrors = 0
				continue
			}

			consecutiveErrors++
			backoff := m.calculateBackoff(consecutiveErrors)

			select {
			case <-m.ctx.Done():
				return
			case <-time.After(backoff):
				continue
			}
		}

		m.routeMessages(results, subs)
	}
}

func (m *multiplexer) calculateBackoff(consecutiveErrors int) time.Duration {
	backoff := m.config.RetryBaseDelay

	for i := 1; i < consecutiveErrors && i < m.config.MaxRetries; i++ {
		backoff *= 2
	}

	if backoff > m.config.RetryMaxDelay {
		backoff = m.config.RetryMaxDelay
	}

	return backoff
}

func (m *multiplexer) stop() {
	m.cancel()
	m.wg.Wait()
}

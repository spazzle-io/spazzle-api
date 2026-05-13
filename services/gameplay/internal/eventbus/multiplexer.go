package eventbus

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

const (
	idlePollInterval    = 500 * time.Millisecond
	streamMessageBuffer = 256
)

type subscription struct {
	streamKey string
	lastID    atomic.Value
	handler   MessageHandler
	msgCh     chan redis.XMessage
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
		lastID = m.resolveCurrentStreamID(ctx, streamKey)
	}

	sub := &subscription{
		streamKey: streamKey,
		handler:   handler,
		msgCh:     make(chan redis.XMessage, streamMessageBuffer),
		ctx:       subCtx,
		cancel:    subCancel,
	}
	sub.lastID.Store(lastID)

	m.mu.Lock()
	m.subscriptions[streamKey] = sub
	m.mu.Unlock()

	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		m.startWorker(sub)
	}()

	m.interruptXRead()
}

func (m *multiplexer) startWorker(sub *subscription) {
	for {
		select {
		case <-sub.ctx.Done():
			return
		case msg := <-sub.msgCh:
			decoded, err := decodeMessage(msg.ID, msg.Values)
			if err != nil {
				log.Error().
					Err(err).
					Str("id", msg.ID).
					Any("msg", msg.Values).
					Str("stream_key", sub.streamKey).
					Msg("failed to decode redis event message")
				continue
			}

			sub.handler(sub.ctx, decoded)
		}
	}
}

func (m *multiplexer) resolveCurrentStreamID(ctx context.Context, streamKey string) string {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	msgs, err := m.client.XRevRangeN(ctx, streamKey, "+", "-", 1).Result()
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

		lastID, ok := sub.lastID.Load().(string)
		if !ok {
			lastID = "0"
		}

		subs[streamKey] = sub
		streams = append(streams, streamKey)
		lastIDs = append(lastIDs, lastID)
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

		for _, msg := range stream.Messages {
			sub.lastID.Store(msg.ID)
			select {
			case sub.msgCh <- msg:
			default:
				log.Warn().Str("stream", stream.Stream).Msg("subscription msg channel full, dropping message")
			}
		}
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
	m.mu.Lock()
	for _, sub := range m.subscriptions {
		sub.cancel()
	}
	m.mu.Unlock()

	m.cancel()
	m.wg.Wait()
}

package workflow

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/eventbus"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameevents"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow/internal/activities"
	"go.temporal.io/sdk/workflow"
)

const EventDeliveryTimeout = time.Second * 5

type sendGameEventConfig struct {
	streamType     eventbus.StreamType
	targetClientID uuid.UUID
	waitForAck     bool
	marker         eventbus.Marker
}

type GameEvent[T any] struct {
	TargetClientID uuid.UUID
	Payload        T
}

type SendGameEventOption func(*sendGameEventConfig)

func WithTargetClient(id uuid.UUID) SendGameEventOption {
	return func(c *sendGameEventConfig) {
		c.targetClientID = id
	}
}

func WithAck() SendGameEventOption {
	return func(c *sendGameEventConfig) {
		c.waitForAck = true
	}
}

func WithStreamType(st eventbus.StreamType) SendGameEventOption {
	return func(c *sendGameEventConfig) {
		c.streamType = st
	}
}

func WithMarker(m eventbus.Marker) SendGameEventOption {
	return func(c *sendGameEventConfig) {
		c.marker = m
	}
}

func newSendGameEventConfig(opts []SendGameEventOption) sendGameEventConfig {
	cfg := sendGameEventConfig{
		streamType: eventbus.GameEventsStreamType,
	}
	for _, o := range opts {
		o(&cfg)
	}

	return cfg
}

func sendGameEvent[T any](
	ctx workflow.Context,
	state *GameState,
	notifyCh workflow.Channel,
	eventType string,
	payload T,
	opts ...SendGameEventOption,
) (eventDelivered bool, err error) {
	cfg := newSendGameEventConfig(opts)

	correlationID, err := generateUUID(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to generate correlation ID: %w", err)
	}

	if cfg.waitForAck {
		state.PendingAcks[correlationID] = &PendingAck{
			CorrelationID: correlationID,
			ReceivedFrom:  make(map[uuid.UUID]gameevents.AckStatus),
			CreatedAt:     workflow.Now(ctx).UTC(),
		}
		defer delete(state.PendingAcks, correlationID)
	}

	var a *activities.Activities
	var publishGameEventResult activities.PublishGameEventsResult

	publishGameEventsParams := activities.PublishGameEventsParams{
		GameServerID: getGameServerID(ctx),
		GameID:       state.GameID,
		StreamType:   cfg.streamType,
		EventType:    eventType,
		Events: []activities.GameEventEntry{
			{
				TargetClientID: cfg.targetClientID,
				CorrelationID:  correlationID,
				EventPayload:   payload,
				Marker:         cfg.marker,
			},
		},
	}

	err = workflow.ExecuteActivity(ctx, a.PublishGameEvents, publishGameEventsParams).Get(ctx, &publishGameEventResult)
	if err != nil {
		return false, fmt.Errorf("failed to publish game event: %w", err)
	}

	if cfg.waitForAck {
		return waitForEventAck(ctx, state, correlationID, EventDeliveryTimeout, notifyCh)
	}

	return false, nil
}

func sendGameEvents[T any](
	ctx workflow.Context,
	state *GameState,
	eventType string,
	events []GameEvent[T],
	opts ...SendGameEventOption,
) error {
	cfg := newSendGameEventConfig(opts)

	entries := make([]activities.GameEventEntry, len(events))
	for i, event := range events {
		correlationID, err := generateUUID(ctx)
		if err != nil {
			return fmt.Errorf("failed to generate correlation ID: %w", err)
		}

		entries[i] = activities.GameEventEntry{
			TargetClientID: event.TargetClientID,
			CorrelationID:  correlationID,
			EventPayload:   event.Payload,
			Marker:         cfg.marker,
		}
	}

	var a *activities.Activities
	var publishGameEventResult activities.PublishGameEventsResult

	publishGameEventsParams := activities.PublishGameEventsParams{
		GameServerID: getGameServerID(ctx),
		GameID:       state.GameID,
		StreamType:   cfg.streamType,
		EventType:    eventType,
		Events:       entries,
	}

	err := workflow.ExecuteActivity(ctx, a.PublishGameEvents, publishGameEventsParams).Get(ctx, &publishGameEventResult)
	if err != nil {
		return fmt.Errorf("failed to publish game events: %w", err)
	}

	return nil
}

func waitForEventAck(
	ctx workflow.Context,
	state *GameState,
	correlationID uuid.UUID,
	deliveryTimeout time.Duration,
	notifyCh workflow.Channel,
) (eventDelivered bool, err error) {
	pendingAck := state.PendingAcks[correlationID]
	numGameServerInstances := len(state.GameServerInstances)

	if numGameServerInstances == 0 {
		return false, errors.New("no game server instances registered")
	}

	timerCtx, cancelTimer := workflow.WithCancel(ctx)
	defer cancelTimer()

	timerFuture := workflow.NewTimer(timerCtx, deliveryTimeout)
	timedOut := false

	for {
		for _, instanceID := range sortedUUIDs(pendingAck.ReceivedFrom) {
			switch pendingAck.ReceivedFrom[instanceID] {
			case gameevents.AckStatusDelivered:
				return true, nil
			case gameevents.AckStatusFailed:
				return false, errors.New("event not delivered successfully")
			}
		}

		if len(pendingAck.ReceivedFrom) >= numGameServerInstances {
			return false, nil
		}

		if timedOut {
			return false, errors.New("timed out waiting for event acknowledgement")
		}

		selector := workflow.NewSelector(ctx)

		selector.AddReceive(notifyCh, func(c workflow.ReceiveChannel, more bool) {
			var tmp struct{}
			c.Receive(ctx, &tmp)
		})

		selector.AddFuture(timerFuture, func(f workflow.Future) {
			timedOut = true
		})

		selector.Select(ctx)
	}
}

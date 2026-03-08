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

type sendGameEventOpts struct {
	StreamType     eventbus.StreamType
	TargetClientID uuid.UUID
	WaitForAck     bool
	Marker         eventbus.Marker
}

func waitForEventAck(
	ctx workflow.Context,
	state *GameState,
	correlationID uuid.UUID,
	deliveryTimeout time.Duration,
	notifyCh workflow.Channel,
) (eventDelivered bool, err error) {
	pendingAcks := state.PendingAcks[correlationID]
	numGameServerInstances := len(state.GameServerInstances)

	if numGameServerInstances == 0 {
		return false, errors.New("no game server instances registered")
	}

	timerCtx, cancelTimer := workflow.WithCancel(ctx)
	defer cancelTimer()

	timerFuture := workflow.NewTimer(timerCtx, deliveryTimeout)
	timedOut := false

	for {
		for _, instanceID := range sortedUUIDs(pendingAcks.ReceivedFrom) {
			ackStatus := pendingAcks.ReceivedFrom[instanceID]

			if ackStatus == gameevents.AckStatusDelivered {
				return true, nil
			}

			if ackStatus == gameevents.AckStatusFailed {
				return false, errors.New("event not delivered successfully")
			}
		}

		if len(pendingAcks.ReceivedFrom) >= numGameServerInstances {
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

func sendGameEvent[T any](
	ctx workflow.Context,
	state *GameState,
	notifyCh workflow.Channel,
	eventType string,
	payload T,
	opts *sendGameEventOpts,
) (eventDelivered bool, err error) {
	o := sendGameEventOpts{
		StreamType: eventbus.GameEventsStreamType,
	}
	if opts != nil {
		if opts.StreamType != "" {
			o.StreamType = opts.StreamType
		}
		o.TargetClientID = opts.TargetClientID
		o.WaitForAck = opts.WaitForAck
		o.Marker = opts.Marker
	}

	correlationID, err := generateUUID(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to generate correlation ID: %w", err)
	}

	if o.WaitForAck {
		state.PendingAcks[correlationID] = &PendingAck{
			CorrelationID: correlationID,
			ReceivedFrom:  make(map[uuid.UUID]gameevents.AckStatus),
			CreatedAt:     workflow.Now(ctx).UTC(),
		}
		defer delete(state.PendingAcks, correlationID)
	}

	var a *activities.Activities

	publishGameEventParams := activities.PublishGameEventParams{
		GameServerID:   getGameServerID(ctx),
		GameID:         state.GameID,
		StreamType:     o.StreamType,
		TargetClientID: o.TargetClientID,
		CorrelationID:  correlationID,
		EventType:      eventType,
		EventPayload:   payload,
		Marker:         o.Marker,
	}
	var publishGameEventResult activities.PublishGameEventResult

	err = workflow.ExecuteActivity(ctx, a.PublishGameEvent, publishGameEventParams).Get(ctx, &publishGameEventResult)
	if err != nil {
		return false, fmt.Errorf("failed to execute publish game event activity: %w", err)
	}

	if o.WaitForAck {
		return waitForEventAck(ctx, state, correlationID, EventDeliveryTimeout, notifyCh)
	}

	return false, nil
}

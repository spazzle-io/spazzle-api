package activities

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/eventbus"
)

// TODO: Rewrite this activity to accept a list of game event entries for cases where we want
// to send messages of the same type to all/multiple players e.g game/round ended where
// we want to send each player their result rather than broadcasting all results to everyone.
//
// Proposed input structure:
//	type PublishGameEventsParams struct {
//	   GameServerID uuid.UUID
//	   GameID       uuid.UUID
//	   StreamType   eventbus.StreamType
//	   EventType    string
//	   Events       []GameEventEntry
//	}
//
//	type GameEventEntry struct {
//	   TargetClientID uuid.UUID
//	   CorrelationID  uuid.UUID
//	   EventPayload   any
//	   Marker         eventbus.Marker
//	}
//
// Proposed output structure:
//	type PublishGameEventResult struct {
//		MessageID string
//	}
// The workflow already has correlation IDs for each message and doesnt need to reference bus message IDs
// so it can be safely dropped from the output.
//
// When sending batch messages, we also want to use different activity options configs that are
// better suited for a long running activity with limited number of retries e.g
//	bulkAO := workflow.ActivityOptions{
//		StartToCloseTimeout: 5 * time.Minute,
//		HeartbeatTimeout:    30 * time.Second,
//		RetryPolicy: &temporal.RetryPolicy{
//			InitialInterval:    2 * time.Second,
//			MaximumInterval:    30 * time.Second,
//			BackoffCoefficient: 2,
//			MaximumAttempts:    3,
//		},
//	}
//

type PublishGameEventParams struct {
	GameServerID   uuid.UUID
	GameID         uuid.UUID
	StreamType     eventbus.StreamType
	TargetClientID uuid.UUID
	CorrelationID  uuid.UUID
	EventType      string
	EventPayload   any
	Marker         eventbus.Marker
}

type PublishGameEventResult struct {
	MessageID string
}

func (a *Activities) PublishGameEvent(
	ctx context.Context,
	params PublishGameEventParams,
) (*PublishGameEventResult, error) {
	payload, err := json.Marshal(params.EventPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal game event payload: %w of type: %s", err, params.EventType)
	}

	eventMsg := eventbus.PublishMessage{
		Type:           params.EventType,
		Payload:        payload,
		TargetClientID: params.TargetClientID,
		CorrelationID:  params.CorrelationID,
		Marker:         params.Marker,
	}

	game := eventbus.GameIdentifier{
		GameID:       params.GameID,
		GameServerID: params.GameServerID,
	}

	session, err := a.Bus.Session(game)
	if err != nil {
		return nil, err
	}

	messageID, err := session.Publish(ctx, params.StreamType, eventMsg)

	return &PublishGameEventResult{
		MessageID: messageID,
	}, err
}

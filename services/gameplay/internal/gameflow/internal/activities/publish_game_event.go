package activities

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/eventbus"
)

type GameEventEntry struct {
	TargetClientID uuid.UUID
	CorrelationID  uuid.UUID
	EventPayload   any
	Marker         eventbus.Marker
}

type PublishGameEventsParams struct {
	GameServerID uuid.UUID
	GameID       uuid.UUID
	StreamType   eventbus.StreamType
	EventType    string
	Events       []GameEventEntry
}

type PublishGameEventsResult struct{}

func (a *Activities) PublishGameEvents(
	ctx context.Context,
	params PublishGameEventsParams,
) (*PublishGameEventsResult, error) {
	game := eventbus.GameIdentifier{
		GameID:       params.GameID,
		GameServerID: params.GameServerID,
	}

	session, err := a.Bus.Session(game)
	if err != nil {
		return nil, err
	}

	for _, event := range params.Events {
		payload, err := json.Marshal(event.EventPayload)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal game event payload of type: %s: %w", params.EventType, err)
		}

		eventMsg := eventbus.PublishMessage{
			Type:           params.EventType,
			Payload:        payload,
			TargetClientID: event.TargetClientID,
			CorrelationID:  event.CorrelationID,
			Marker:         event.Marker,
		}

		_, err = session.Publish(ctx, params.StreamType, eventMsg)
		if err != nil {
			return nil, fmt.Errorf("failed to publish event: %w", err)
		}
	}

	return &PublishGameEventsResult{}, err
}

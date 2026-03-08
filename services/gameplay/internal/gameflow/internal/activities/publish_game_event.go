package activities

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/eventbus"
)

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

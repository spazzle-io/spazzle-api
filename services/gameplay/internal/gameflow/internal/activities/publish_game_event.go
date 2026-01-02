package activities

import (
	"context"

	"github.com/google/uuid"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/eventbus"
)

type PublishGameEventParams struct {
	GameServerID uuid.UUID
	GameID       uuid.UUID
	StreamType   eventbus.StreamType
	Msg          eventbus.PublishMessage
}

type PublishGameEventResult struct {
	MessageID string
}

func (a *Activities) PublishGameEvent(
	ctx context.Context,
	params PublishGameEventParams,
) (*PublishGameEventResult, error) {
	game := eventbus.GameIdentifier{
		GameID:       params.GameID,
		GameServerID: params.GameServerID,
	}

	session, err := a.Bus.Session(game)
	if err != nil {
		return nil, err
	}

	messageID, err := session.Publish(ctx, params.StreamType, params.Msg)

	return &PublishGameEventResult{
		MessageID: messageID,
	}, err
}

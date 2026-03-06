package activities

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/eventbus"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameevents"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestPublishGameEvent(t *testing.T) {
	deps := setupActivities(t)

	params := PublishGameEventParams{
		GameServerID:   uuid.New(),
		GameID:         uuid.New(),
		StreamType:     eventbus.GameEventsStreamType,
		TargetClientID: uuid.New(),
		CorrelationID:  uuid.New(),
		EventType:      gameevents.TypeBeginDrawing,
		EventPayload: &gameevents.BeginDrawingPayload{
			CurrentRound: uint8(1),
			EndsAt:       time.Now().UTC(),
		},
	}

	marshaledPayload, err := json.Marshal(params.EventPayload)
	require.NoError(t, err)
	require.NotEmpty(t, marshaledPayload)

	eventMsg := eventbus.PublishMessage{
		Type:           params.EventType,
		Payload:        marshaledPayload,
		TargetClientID: params.TargetClientID,
		CorrelationID:  params.CorrelationID,
	}

	game := eventbus.GameIdentifier{
		GameID:       params.GameID,
		GameServerID: params.GameServerID,
	}

	deps.Bus.EXPECT().
		Session(gomock.Eq(game)).
		Times(1).
		Return(deps.Session, nil)

	deps.Session.EXPECT().
		Publish(gomock.Any(), gomock.Eq(eventbus.GameEventsStreamType), gomock.Eq(eventMsg)).
		Times(1).
		Return("msg-id", nil)

	expectedResult := &PublishGameEventResult{
		MessageID: "msg-id",
	}

	result, err := deps.Activities.PublishGameEvent(context.Background(), params)
	require.NoError(t, err)
	require.Equal(t, expectedResult, result)
}

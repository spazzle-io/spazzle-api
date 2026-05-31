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

	params := PublishGameEventsParams{
		GameServerID: uuid.New(),
		GameID:       uuid.New(),
		StreamType:   eventbus.GameEventsStreamType,
		EventType:    gameevents.TypeBeginDrawing,
		Events: []GameEventEntry{
			{
				TargetClientID: uuid.New(),
				CorrelationID:  uuid.New(),
				EventPayload: &gameevents.BeginDrawingPayload{
					CurrentRound: uint8(1),
					EndsAt:       time.Now().UTC(),
				},
				Marker: eventbus.Marker{
					Round: uint8(3),
					Type:  eventbus.MarkerRoundEnded,
				},
			},
		},
	}

	marshaledPayload, err := json.Marshal(params.Events[0].EventPayload)
	require.NoError(t, err)
	require.NotEmpty(t, marshaledPayload)

	eventMsg := eventbus.PublishMessage{
		Type:           params.EventType,
		Payload:        marshaledPayload,
		TargetClientID: params.Events[0].TargetClientID,
		CorrelationID:  params.Events[0].CorrelationID,
		Marker:         params.Events[0].Marker,
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

	expectedResult := &PublishGameEventsResult{}

	result, err := deps.Activities.PublishGameEvents(context.Background(), params)
	require.NoError(t, err)
	require.Equal(t, expectedResult, result)
}

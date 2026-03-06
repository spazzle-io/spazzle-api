package workflow

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/google/uuid"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameevents"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow/internal/activities"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow/types"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/client"
)

type EndGameTestSuite struct {
	WorkflowTestSuite
}

func TestEndGame(t *testing.T) {
	suite.Run(t, new(EndGameTestSuite))
}

func (s *EndGameTestSuite) TestEndGame() {
	s.env.OnActivity(s.activities.SelectRandomWord, mock.Anything, mock.Anything).
		Maybe().
		Return(&activities.SelectRandomWordResult{
			Word: gofakeit.Word(),
		}, nil)

	serverID := uuid.New()
	instanceID := uuid.New()

	gameID := uuid.New()
	player1 := uuid.New()
	player2 := uuid.New()

	var capturedState *types.GameStateView

	input := types.GameInput{
		GameID:          gameID,
		NumRounds:       10,
		DrawingDuration: 60 * time.Second,
		StakePerGame:    "1000000000000000000",
	}

	s.env.RegisterDelayedCallback(func() {
		s.env.SignalWorkflow(SignalGameServerInstanceHeartbeat, GameServerInstanceHeartbeatSignal{
			InstanceID: instanceID,
		})
	}, 100*time.Millisecond)

	s.env.RegisterDelayedCallback(func() {
		s.env.SignalWorkflow(SignalPlayersJoin, PlayersJoinSignal{
			GameID:    gameID,
			PlayerIDs: []uuid.UUID{player1, player2},
		})
	}, 200*time.Millisecond)

	sentGameEndedEvent := false
	s.env.OnActivity(s.activities.PublishGameEvent, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			params := args.Get(1).(activities.PublishGameEventParams)

			if params.EventType == gameevents.TypeGameEnded {
				sentGameEndedEvent = true

				var payload gameevents.GameEndedPayload
				raw := params.EventPayload.(map[string]interface{})
				b, err := json.Marshal(raw)
				s.Require().NoError(err)
				s.Require().NoError(json.Unmarshal(b, &payload))

				s.Equal(uint8(10), payload.TotalRounds)
				s.Equal("2000000000000000000", payload.TotalPot)
				s.NotEmpty(payload.Results)
				s.Len(payload.Results, 2)
			}

			if params.TargetClientID == uuid.Nil {
				return
			}

			s.env.SignalWorkflow(SignalEventAck, gameevents.EventAckPayload{
				CorrelationID: params.CorrelationID,
				InstanceID:    instanceID,
				Status:        gameevents.AckStatusDelivered,
			})
		}).
		Return(func(ctx context.Context, params activities.PublishGameEventParams) (*activities.PublishGameEventResult, error) {
			if params.EventType == gameevents.TypeGameEnded {
				val, err := s.env.QueryWorkflow(QueryGetGameState)
				s.NoError(err)

				var state types.GameStateView
				s.NoError(val.Get(&state))
				capturedState = &state
			}

			return &activities.PublishGameEventResult{
				MessageID: "some-message-id",
			}, nil
		})

	s.env.SetStartWorkflowOptions(client.StartWorkflowOptions{
		ID: serverID.String(),
	})
	s.env.ExecuteWorkflow(GameWorkflow, input)

	s.True(sentGameEndedEvent)
	s.Equal(gameID, capturedState.GameID)
	s.Equal(types.PhaseEndGame, capturedState.Phase)
	s.Equal(types.SubPhaseNone, capturedState.SubPhase)
	s.Empty(capturedState.CurrentArtist)
	s.Empty(capturedState.CurrentWord)
	s.Len(capturedState.Players, 2)
}

func TestGetFinalPlayerResults(t *testing.T) {
	player1 := &PlayerGameState{
		PlayerID:     uuid.New(),
		Points:       80,
		StakeLost:    "200000000000000",
		RoundsPlayed: 5,
		IsEjected:    false,
	}

	player2 := &PlayerGameState{
		PlayerID:     uuid.New(),
		Points:       100,
		StakeLost:    "200000000000000",
		RoundsPlayed: 3,
		IsEjected:    false,
	}

	player3 := &PlayerGameState{
		PlayerID:     uuid.New(),
		Points:       80,
		StakeLost:    "100000000000000",
		RoundsPlayed: 3,
		IsEjected:    false,
	}

	player4 := &PlayerGameState{
		PlayerID:     uuid.New(),
		Points:       90,
		StakeLost:    "250000000000000",
		RoundsPlayed: 3,
		IsEjected:    true,
	}

	player5 := &PlayerGameState{
		PlayerID:     uuid.New(),
		Points:       30,
		StakeLost:    "120000000000000",
		RoundsPlayed: 1,
		IsEjected:    false,
	}

	state := &GameState{
		GamePot: "600000000000000",
		Players: map[uuid.UUID]*PlayerGameState{
			player1.PlayerID: player1,
			player2.PlayerID: player2,
			player3.PlayerID: player3,
			player4.PlayerID: player4,
			player5.PlayerID: player5,
		},
	}

	finalResults := getFinalPlayerResults(state)

	expectedFinalResults := []*gameevents.PlayerFinalResult{
		{
			PlayerID:          player2.PlayerID,
			Position:          1,
			TotalPoints:       100,
			TotalStakeLost:    "200000000000000",
			RoundsPlayed:      3,
			IsEjected:         false,
			ProvisionalPayout: "142105263157894",
		},
		{
			PlayerID:          player4.PlayerID,
			Position:          2,
			TotalPoints:       90,
			TotalStakeLost:    "250000000000000",
			RoundsPlayed:      3,
			IsEjected:         true,
			ProvisionalPayout: "127894736842105",
		},
		{
			PlayerID:          player1.PlayerID,
			Position:          3,
			TotalPoints:       80,
			TotalStakeLost:    "200000000000000",
			RoundsPlayed:      5,
			IsEjected:         false,
			ProvisionalPayout: "113684210526315",
		},
		{
			PlayerID:          player3.PlayerID,
			Position:          3,
			TotalPoints:       80,
			TotalStakeLost:    "100000000000000",
			RoundsPlayed:      3,
			IsEjected:         false,
			ProvisionalPayout: "113684210526315",
		},
		{
			PlayerID:          player5.PlayerID,
			Position:          5,
			TotalPoints:       30,
			TotalStakeLost:    "120000000000000",
			RoundsPlayed:      1,
			IsEjected:         false,
			ProvisionalPayout: "42631578947368",
		},
	}

	require.NotEmpty(t, finalResults)
	require.Len(t, finalResults, 5)
	require.ElementsMatch(t, expectedFinalResults, finalResults)
}

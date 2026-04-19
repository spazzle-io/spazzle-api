package workflow

import (
	"testing"
	"time"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow/internal/activities"
	"github.com/stretchr/testify/mock"

	"github.com/google/uuid"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow/types"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/client"
)

type PhaseWaitingTestSuite struct {
	WorkflowTestSuite
}

func TestPhaseWaiting(t *testing.T) {
	suite.Run(t, new(PhaseWaitingTestSuite))
}

func (s *PhaseWaitingTestSuite) TestTransitionsToPhasePrepareRound() {
	s.env.OnActivity(s.activities.PublishGameEvents, mock.Anything, mock.Anything).
		Maybe().
		Return(&activities.PublishGameEventsResult{}, nil)
	s.env.OnActivity(s.activities.SelectRandomWord, mock.Anything, mock.Anything).
		Maybe().
		Return(&activities.SelectRandomWordResult{
			Word: gofakeit.Word(),
		}, nil)
	s.env.OnActivity(s.activities.ArchiveGame, mock.Anything, mock.Anything).
		Maybe().
		Return(&activities.ArchiveGameResult{}, nil)

	serverID := uuid.New()
	gameID := uuid.New()
	player1 := uuid.New()
	player2 := uuid.New()

	input := types.GameInput{
		GameID:          gameID,
		NumRounds:       10,
		DrawingDuration: 60 * time.Second,
		StakePerGame:    "1000000000000000000",
	}

	s.env.RegisterDelayedCallback(func() {
		s.env.SignalWorkflow(SignalPlayersJoin, PlayersJoinSignal{
			GameID:    gameID,
			PlayerIDs: []uuid.UUID{player1, player2},
		})
	}, 100*time.Millisecond)

	var capturedState *types.GameStateView
	s.env.RegisterDelayedCallback(func() {
		val, err := s.env.QueryWorkflow(QueryGetGameState)
		s.NoError(err)

		var state types.GameStateView
		s.NoError(val.Get(&state))
		capturedState = &state
	}, 150*time.Millisecond)

	s.env.RegisterDelayedCallback(func() {
		s.env.SignalWorkflow(SignalTerminateGame, TerminateGameSignal{
			GameID: gameID,
			Reason: "test",
		})
	}, 200*time.Millisecond)

	s.env.SetStartWorkflowOptions(client.StartWorkflowOptions{
		ID: serverID.String(),
	})
	s.env.ExecuteWorkflow(GameWorkflow, input)

	s.Equal(gameID, capturedState.GameID)
	s.Equal(types.PhasePrepareRound, capturedState.Phase)
	s.Len(capturedState.Players, 2)
}

func (s *PhaseWaitingTestSuite) TestNotEnoughPlayers_StaysInWaitingPhase() {
	s.env.OnActivity(s.activities.PublishGameEvents, mock.Anything, mock.Anything).
		Maybe().
		Return(&activities.PublishGameEventsResult{}, nil)
	s.env.OnActivity(s.activities.SelectRandomWord, mock.Anything, mock.Anything).
		Maybe().
		Return(&activities.SelectRandomWordResult{
			Word: gofakeit.Word(),
		}, nil)
	s.env.OnActivity(s.activities.ArchiveGame, mock.Anything, mock.Anything).
		Maybe().
		Return(&activities.ArchiveGameResult{}, nil)

	serverID := uuid.New()
	gameID := uuid.New()
	player1 := uuid.New()

	input := types.GameInput{
		GameID:          gameID,
		NumRounds:       10,
		DrawingDuration: 60 * time.Second,
		StakePerGame:    "1000000000000000000",
	}

	s.env.RegisterDelayedCallback(func() {
		s.env.SignalWorkflow(SignalPlayersJoin, PlayersJoinSignal{
			GameID:    gameID,
			PlayerIDs: []uuid.UUID{player1},
		})
	}, 100*time.Millisecond)

	var capturedState *types.GameStateView
	s.env.RegisterDelayedCallback(func() {
		val, err := s.env.QueryWorkflow(QueryGetGameState)
		s.NoError(err)

		var state types.GameStateView
		s.NoError(val.Get(&state))
		capturedState = &state
	}, 200*time.Millisecond)

	s.env.RegisterDelayedCallback(func() {
		s.env.SignalWorkflow(SignalTerminateGame, TerminateGameSignal{
			GameID: gameID,
			Reason: "test",
		})
	}, 400*time.Millisecond)

	s.env.SetStartWorkflowOptions(client.StartWorkflowOptions{
		ID: serverID.String(),
	})
	s.env.ExecuteWorkflow(GameWorkflow, input)

	s.Equal(gameID, capturedState.GameID)
	s.Equal(types.PhaseWaiting, capturedState.Phase)
	s.Len(capturedState.Players, 1)
}

func TestHasEnoughPlayers(t *testing.T) {
	testCases := []struct {
		name     string
		state    *GameState
		expected bool
	}{
		{
			name: "has enough players",
			state: &GameState{
				MinNumPlayersToStart: 2,
				Players: map[uuid.UUID]*PlayerGameState{
					uuid.New(): {
						IsConnected: true,
						IsEjected:   false,
					},
					uuid.New(): {
						IsConnected: true,
						IsEjected:   false,
					},
				},
			},
			expected: true,
		},
		{
			name: "not enough players - a player is ejected",
			state: &GameState{
				MinNumPlayersToStart: 2,
				Players: map[uuid.UUID]*PlayerGameState{
					uuid.New(): {
						IsConnected: true,
						IsEjected:   true,
					},
					uuid.New(): {
						IsConnected: true,
						IsEjected:   false,
					},
					uuid.New(): {
						IsConnected: true,
						IsEjected:   true,
					},
					uuid.New(): {
						IsConnected: false,
						IsEjected:   false,
					},
					uuid.New(): {
						IsConnected: false,
						IsEjected:   true,
					},
				},
			},
			expected: false,
		},
		{
			name: "not enough players - a player is disconnected",
			state: &GameState{
				MinNumPlayersToStart: 2,
				Players: map[uuid.UUID]*PlayerGameState{
					uuid.New(): {
						IsConnected: true,
						IsEjected:   false,
					},
					uuid.New(): {
						IsConnected: false,
						IsEjected:   false,
					},
				},
			},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expected, hasEnoughPlayers(tc.state))
		})
	}
}

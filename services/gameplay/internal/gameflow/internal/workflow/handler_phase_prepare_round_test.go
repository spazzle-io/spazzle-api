package workflow

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameevents"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow/internal/activities"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow/types"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/client"
)

type PhasePrepareRoundTestSuite struct {
	WorkflowTestSuite
}

func TestPhasePrepareRound(t *testing.T) {
	suite.Run(t, new(PhasePrepareRoundTestSuite))
}

func (s *PhasePrepareRoundTestSuite) TestPreparesRoundSuccessfully() {
	s.env.OnActivity(s.activities.ArchiveGame, mock.Anything, mock.Anything).
		Maybe().
		Return(&activities.ArchiveGameResult{})

	serverID := uuid.New()
	instanceID := uuid.New()

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

	s.env.OnActivity(s.activities.PublishGameEvent, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			params := args.Get(1).(activities.PublishGameEventParams)

			if params.TargetClientID == uuid.Nil {
				return
			}

			s.Equal(gameevents.TypeArtistSelected, params.EventType)

			s.env.SignalWorkflow(SignalEventAck, gameevents.EventAckPayload{
				CorrelationID: params.CorrelationID,
				InstanceID:    instanceID,
				Status:        gameevents.AckStatusDelivered,
			})

			s.env.SignalWorkflow(SignalTerminateGame, TerminateGameSignal{
				GameID: gameID,
				Reason: "test",
			})
		}).
		Return(&activities.PublishGameEventResult{
			MessageID: "some-message-id",
		}, nil)

	var capturedState *types.GameStateView
	s.env.RegisterDelayedCallback(func() {
		val, err := s.env.QueryWorkflow(QueryGetGameState)
		s.NoError(err)

		var state types.GameStateView
		s.NoError(val.Get(&state))
		capturedState = &state
	}, 300*time.Millisecond)

	s.env.SetStartWorkflowOptions(client.StartWorkflowOptions{
		ID: serverID.String(),
	})
	s.env.ExecuteWorkflow(GameWorkflow, input)

	s.Equal(gameID, capturedState.GameID)
	s.Equal(types.PhaseEndRound, capturedState.Phase)
	s.Empty(capturedState.CurrentArtist)
	s.Len(capturedState.Players, 2)
}

func (s *PhasePrepareRoundTestSuite) TestCouldNotSelectAndNotifyArtist() {
	s.env.OnActivity(s.activities.ArchiveGame, mock.Anything, mock.Anything).
		Maybe().
		Return(&activities.ArchiveGameResult{})

	serverID := uuid.New()
	instanceID := uuid.New()

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

	s.env.OnActivity(s.activities.PublishGameEvent, mock.Anything, mock.Anything).
		Return(&activities.PublishGameEventResult{
			MessageID: "some-message-id",
		}, nil)

	var capturedState *types.GameStateView
	s.env.RegisterDelayedCallback(func() {
		val, err := s.env.QueryWorkflow(QueryGetGameState)
		s.NoError(err)

		var state types.GameStateView
		s.NoError(val.Get(&state))
		capturedState = &state
	}, 300*time.Millisecond)

	s.env.RegisterDelayedCallback(func() {
		s.env.SignalWorkflow(SignalTerminateGame, TerminateGameSignal{
			GameID: gameID,
			Reason: "test",
		})
	}, 350*time.Millisecond)

	s.env.SetStartWorkflowOptions(client.StartWorkflowOptions{
		ID: serverID.String(),
	})
	s.env.ExecuteWorkflow(GameWorkflow, input)

	s.Equal(gameID, capturedState.GameID)
	s.Equal(types.PhasePrepareRound, capturedState.Phase)
	s.Empty(capturedState.CurrentArtist)
	s.Len(capturedState.Players, 2)
}

func (s *PhasePrepareRoundTestSuite) TestNotEnoughPlayersAfterSelectingArtist() {
	s.env.OnActivity(s.activities.ArchiveGame, mock.Anything, mock.Anything).
		Maybe().
		Return(&activities.ArchiveGameResult{})

	serverID := uuid.New()
	instanceID := uuid.New()

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

	s.env.OnActivity(s.activities.PublishGameEvent, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			params := args.Get(1).(activities.PublishGameEventParams)

			if params.TargetClientID == uuid.Nil {
				return
			}

			s.Equal(gameevents.TypeArtistSelected, params.EventType)

			s.env.SignalWorkflow(SignalPlayersLeave, PlayersLeaveSignal{
				PlayerIDs: []uuid.UUID{player1},
			})

			s.env.SignalWorkflow(SignalEventAck, gameevents.EventAckPayload{
				CorrelationID: params.CorrelationID,
				InstanceID:    instanceID,
				Status:        gameevents.AckStatusDelivered,
			})

			s.env.SignalWorkflow(SignalTerminateGame, TerminateGameSignal{
				GameID: gameID,
				Reason: "test",
			})
		}).
		Return(&activities.PublishGameEventResult{
			MessageID: "some-message-id",
		}, nil)

	var capturedState *types.GameStateView
	s.env.RegisterDelayedCallback(func() {
		val, err := s.env.QueryWorkflow(QueryGetGameState)
		s.NoError(err)

		var state types.GameStateView
		s.NoError(val.Get(&state))
		capturedState = &state
	}, 300*time.Millisecond)

	s.env.RegisterDelayedCallback(func() {
		s.env.SignalWorkflow(SignalTerminateGame, TerminateGameSignal{
			GameID: gameID,
			Reason: "test",
		})
	}, 350*time.Millisecond)

	s.env.SetStartWorkflowOptions(client.StartWorkflowOptions{
		ID: serverID.String(),
	})
	s.env.ExecuteWorkflow(GameWorkflow, input)

	s.Equal(gameID, capturedState.GameID)
	s.Equal(types.PhaseEndRound, capturedState.Phase)
	s.Empty(capturedState.CurrentArtist)
	s.Len(capturedState.Players, 1)
}

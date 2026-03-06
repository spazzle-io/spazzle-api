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
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/client"
)

type InRoundTestSuite struct {
	WorkflowTestSuite
}

func TestPhaseInRound(t *testing.T) {
	suite.Run(t, new(InRoundTestSuite))
}

func (s *InRoundTestSuite) TestConfirmsArtist() {
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

	sentConfirmArtistEvent := false
	s.env.OnActivity(s.activities.PublishGameEvent, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			params := args.Get(1).(activities.PublishGameEventParams)

			if params.EventType == gameevents.TypeArtistConfirmed {
				sentConfirmArtistEvent = true

				var payload gameevents.ArtistConfirmedPayload
				raw := params.EventPayload.(map[string]interface{})
				b, err := json.Marshal(raw)
				s.Require().NoError(err)
				s.Require().NoError(json.Unmarshal(b, &payload))

				s.NotEmpty(payload.ArtistID)
				s.Equal(uint8(DefaultRoundNumber), payload.CurrentRound)
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
			if params.EventType == gameevents.TypeArtistConfirmed {
				val, err := s.env.QueryWorkflow(QueryGetGameState)
				s.NoError(err)

				var state types.GameStateView
				s.NoError(val.Get(&state))
				capturedState = &state

				s.env.SignalWorkflow(SignalTerminateGame, TerminateGameSignal{
					GameID: gameID,
					Reason: "test",
				})
			}

			return &activities.PublishGameEventResult{
				MessageID: "some-message-id",
			}, nil
		})

	s.env.SetStartWorkflowOptions(client.StartWorkflowOptions{
		ID: serverID.String(),
	})
	s.env.ExecuteWorkflow(GameWorkflow, input)

	s.True(sentConfirmArtistEvent)
	s.Equal(gameID, capturedState.GameID)
	s.Equal(types.PhaseInRound, capturedState.Phase)
	s.Equal(types.SubPhaseConfirmArtist, capturedState.SubPhase)
	s.NotEmpty(capturedState.CurrentArtist)
	s.Len(capturedState.Players, 2)
}

func (s *InRoundTestSuite) TestWordSelected_AfterTimeout() {
	serverID := uuid.New()
	instanceID := uuid.New()

	gameID := uuid.New()
	player1 := uuid.New()
	player2 := uuid.New()

	var sentBeginWordSelectionEvent bool
	var capturedState *types.GameStateView

	selectedWord := "🤘🏿🚀 love"
	selectedWordTokens := []types.Token{
		{
			Text:   "🤘🏿🚀",
			Length: 2,
		},
		{
			Text:   "love",
			Length: 4,
		},
	}

	s.env.OnActivity(s.activities.SelectRandomWord, mock.Anything, mock.Anything).
		Maybe().
		Return(&activities.SelectRandomWordResult{
			Word: selectedWord,
		}, nil)

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

			if params.EventType == gameevents.TypeBeginWordSelection {
				sentBeginWordSelectionEvent = true
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
			if params.EventType == gameevents.TypeWordSelected {
				val, err := s.env.QueryWorkflow(QueryGetGameState)
				s.NoError(err)

				var state types.GameStateView
				s.NoError(val.Get(&state))
				capturedState = &state

				s.env.SignalWorkflow(SignalTerminateGame, TerminateGameSignal{
					GameID: gameID,
					Reason: "test",
				})
			}

			return &activities.PublishGameEventResult{
				MessageID: "some-message-id",
			}, nil
		})

	s.env.SetStartWorkflowOptions(client.StartWorkflowOptions{
		ID: serverID.String(),
	})
	s.env.ExecuteWorkflow(GameWorkflow, input)

	s.True(sentBeginWordSelectionEvent)
	s.Equal(gameID, capturedState.GameID)
	s.Equal(types.PhaseInRound, capturedState.Phase)
	s.Equal(types.SubPhaseWordSelection, capturedState.SubPhase)
	s.NotEmpty(capturedState.CurrentArtist)
	s.NotEmpty(capturedState.CurrentWord)
	s.Equal(selectedWord, capturedState.CurrentWord.Text)
	s.Equal(selectedWordTokens, capturedState.CurrentWord.Tokens)
	s.Len(capturedState.Players, 2)
}

func (s *InRoundTestSuite) TestWordSelected_ArtistProvided() {
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

	var sentBeginWordSelectionEvent bool
	var capturedState *types.GameStateView

	selectedWord := "mama beary"
	selectedWordTokens := []types.Token{
		{
			Text:   "mama",
			Length: 4,
		},
		{
			Text:   "beary",
			Length: 5,
		},
	}

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

			if params.EventType == gameevents.TypeBeginWordSelection {
				sentBeginWordSelectionEvent = true

				s.env.SignalWorkflow(SignalWordSelected, WordSelectedSignal{
					GameID:       gameID,
					CurrentRound: 1,
					Word:         selectedWord,
				})
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
			if params.EventType == gameevents.TypeWordSelected {
				val, err := s.env.QueryWorkflow(QueryGetGameState)
				s.NoError(err)

				var state types.GameStateView
				s.NoError(val.Get(&state))
				capturedState = &state

				s.env.SignalWorkflow(SignalTerminateGame, TerminateGameSignal{
					GameID: gameID,
					Reason: "test",
				})
			}

			return &activities.PublishGameEventResult{
				MessageID: "some-message-id",
			}, nil
		})

	s.env.SetStartWorkflowOptions(client.StartWorkflowOptions{
		ID: serverID.String(),
	})
	s.env.ExecuteWorkflow(GameWorkflow, input)

	s.True(sentBeginWordSelectionEvent)
	s.Equal(gameID, capturedState.GameID)
	s.Equal(types.PhaseInRound, capturedState.Phase)
	s.Equal(types.SubPhaseWordSelection, capturedState.SubPhase)
	s.NotEmpty(capturedState.CurrentArtist)
	s.NotEmpty(capturedState.CurrentWord)
	s.Equal(selectedWord, capturedState.CurrentWord.Text)
	s.Equal(selectedWordTokens, capturedState.CurrentWord.Tokens)
	s.Len(capturedState.Players, 2)
}

func (s *InRoundTestSuite) TestBeginDrawingEventSent() {
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

	var sentBeginDrawingEvent bool
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

	s.env.OnActivity(s.activities.PublishGameEvent, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			params := args.Get(1).(activities.PublishGameEventParams)

			if params.EventType == gameevents.TypeBeginDrawing {
				sentBeginDrawingEvent = true
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
			if params.EventType == gameevents.TypeBeginDrawing {
				val, err := s.env.QueryWorkflow(QueryGetGameState)
				s.NoError(err)

				var state types.GameStateView
				s.NoError(val.Get(&state))
				capturedState = &state

				s.env.SignalWorkflow(SignalTerminateGame, TerminateGameSignal{
					GameID: gameID,
					Reason: "test",
				})
			}

			return &activities.PublishGameEventResult{
				MessageID: "some-message-id",
			}, nil
		})

	s.env.SetStartWorkflowOptions(client.StartWorkflowOptions{
		ID: serverID.String(),
	})
	s.env.ExecuteWorkflow(GameWorkflow, input)

	s.True(sentBeginDrawingEvent)
	s.Equal(gameID, capturedState.GameID)
	s.Equal(types.PhaseInRound, capturedState.Phase)
	s.Equal(types.SubPhaseDrawing, capturedState.SubPhase)
	s.NotEmpty(capturedState.CurrentArtist)
	s.NotEmpty(capturedState.CurrentWord)
	s.Len(capturedState.Players, 2)
}

func (s *InRoundTestSuite) TestEndDrawingEventSent() {
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

	var sentEndDrawingEvent bool
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

	s.env.OnActivity(s.activities.PublishGameEvent, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			params := args.Get(1).(activities.PublishGameEventParams)

			if params.EventType == gameevents.TypeEndDrawing {
				sentEndDrawingEvent = true
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
			if params.EventType == gameevents.TypeEndDrawing {
				val, err := s.env.QueryWorkflow(QueryGetGameState)
				s.NoError(err)

				var state types.GameStateView
				s.NoError(val.Get(&state))
				capturedState = &state

				s.env.SignalWorkflow(SignalTerminateGame, TerminateGameSignal{
					GameID: gameID,
					Reason: "test",
				})
			}

			return &activities.PublishGameEventResult{
				MessageID: "some-message-id",
			}, nil
		})

	s.env.SetStartWorkflowOptions(client.StartWorkflowOptions{
		ID: serverID.String(),
	})
	s.env.ExecuteWorkflow(GameWorkflow, input)

	s.True(sentEndDrawingEvent)
	s.Equal(gameID, capturedState.GameID)
	s.Equal(types.PhaseInRound, capturedState.Phase)
	s.Equal(types.SubPhaseDrawing, capturedState.SubPhase)
	s.NotEmpty(capturedState.CurrentArtist)
	s.NotEmpty(capturedState.CurrentWord)
	s.Len(capturedState.Players, 2)
}

func (s *InRoundTestSuite) TestHandlesCorrectGuesses() {
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

	s.env.OnActivity(s.activities.PublishGameEvent, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			params := args.Get(1).(activities.PublishGameEventParams)

			if params.EventType == gameevents.TypeBeginDrawing {
				s.env.SignalWorkflow(SignalCorrectGuesses, CorrectGuessesSignal{
					GameID:       gameID,
					CurrentRound: DefaultRoundNumber,
					Guesses: []types.CorrectGuess{
						{
							PlayerID:  player1,
							Timestamp: time.Now().UTC(),
						},
						{
							PlayerID:  player2,
							Timestamp: time.Now().UTC(),
						},
					},
				})
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
			if params.EventType == gameevents.TypeEndDrawing {
				val, err := s.env.QueryWorkflow(QueryGetGameState)
				s.NoError(err)

				var state types.GameStateView
				s.NoError(val.Get(&state))
				capturedState = &state

				s.env.SignalWorkflow(SignalTerminateGame, TerminateGameSignal{
					GameID: gameID,
					Reason: "test",
				})
			}

			return &activities.PublishGameEventResult{
				MessageID: "some-message-id",
			}, nil
		})

	s.env.SetStartWorkflowOptions(client.StartWorkflowOptions{
		ID: serverID.String(),
	})
	s.env.ExecuteWorkflow(GameWorkflow, input)

	s.Equal(gameID, capturedState.GameID)
	s.Equal(types.PhaseInRound, capturedState.Phase)
	s.Equal(types.SubPhaseDrawing, capturedState.SubPhase)
	s.NotEmpty(capturedState.CurrentArtist)
	s.NotEmpty(capturedState.CurrentWord)
	s.Equal(2, capturedState.NumCorrectGuesses[DefaultRoundNumber])
	s.Len(capturedState.Players, 2)
}

func (s *InRoundTestSuite) TestSelectsNextArtist() {
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

	var sentNextArtistSelectedEvent bool
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

	s.env.OnActivity(s.activities.PublishGameEvent, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			params := args.Get(1).(activities.PublishGameEventParams)

			if params.EventType == gameevents.TypeNextArtistSelected {
				sentNextArtistSelectedEvent = true

				var payload gameevents.NextArtistSelectedPayload
				raw := params.EventPayload.(map[string]interface{})
				b, err := json.Marshal(raw)
				s.Require().NoError(err)
				s.Require().NoError(json.Unmarshal(b, &payload))

				s.Equal(uint8(DefaultRoundNumber+1), payload.Round)
				s.Contains([]uuid.UUID{player1, player2}, payload.PlayerID)
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
			if params.EventType == gameevents.TypeEndDrawing {
				val, err := s.env.QueryWorkflow(QueryGetGameState)
				s.NoError(err)

				var state types.GameStateView
				s.NoError(val.Get(&state))
				capturedState = &state

				s.env.SignalWorkflow(SignalTerminateGame, TerminateGameSignal{
					GameID: gameID,
					Reason: "test",
				})
			}

			return &activities.PublishGameEventResult{
				MessageID: "some-message-id",
			}, nil
		})

	s.env.SetStartWorkflowOptions(client.StartWorkflowOptions{
		ID: serverID.String(),
	})
	s.env.ExecuteWorkflow(GameWorkflow, input)

	s.True(sentNextArtistSelectedEvent)
	s.Equal(gameID, capturedState.GameID)
	s.Equal(types.PhaseInRound, capturedState.Phase)
	s.Equal(types.SubPhaseDrawing, capturedState.SubPhase)
	s.NotEmpty(capturedState.CurrentArtist)
	s.NotEmpty(capturedState.CurrentWord)
	s.Len(capturedState.Players, 2)
}

func (s *InRoundTestSuite) TestSendsWordHints() {
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

	var sentWordHintEvent bool
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

	s.env.OnActivity(s.activities.PublishGameEvent, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			params := args.Get(1).(activities.PublishGameEventParams)

			if params.EventType == gameevents.TypeWordHintRevealed {
				sentWordHintEvent = true

				var payload gameevents.WordHintRevealedPayload
				raw := params.EventPayload.(map[string]interface{})
				b, err := json.Marshal(raw)
				s.Require().NoError(err)
				s.Require().NoError(json.Unmarshal(b, &payload))

				s.Equal(uint8(DefaultRoundNumber), payload.CurrentRound)
				s.NotEmpty(payload.WordHint)
				s.GreaterOrEqual(payload.WordHint.WordTokenIdx, 0)
				s.GreaterOrEqual(payload.WordHint.CharIdx, 0)
				s.NotZero(payload.WordHint.Char)
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
			if params.EventType == gameevents.TypeEndDrawing {
				val, err := s.env.QueryWorkflow(QueryGetGameState)
				s.NoError(err)

				var state types.GameStateView
				s.NoError(val.Get(&state))
				capturedState = &state

				s.env.SignalWorkflow(SignalTerminateGame, TerminateGameSignal{
					GameID: gameID,
					Reason: "test",
				})
			}

			return &activities.PublishGameEventResult{
				MessageID: "some-message-id",
			}, nil
		})

	s.env.SetStartWorkflowOptions(client.StartWorkflowOptions{
		ID: serverID.String(),
	})
	s.env.ExecuteWorkflow(GameWorkflow, input)

	s.True(sentWordHintEvent)
	s.Equal(gameID, capturedState.GameID)
	s.Equal(types.PhaseInRound, capturedState.Phase)
	s.Equal(types.SubPhaseDrawing, capturedState.SubPhase)
	s.NotEmpty(capturedState.CurrentArtist)
	s.NotEmpty(capturedState.CurrentWord)
	s.Len(capturedState.Players, 2)
}

func (s *InRoundTestSuite) TestHandlesArtistDisconnect_ArtistDisconnectSignalSent() {
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

	var artistID uuid.UUID
	var sentArtistDisconnectedEvent bool
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

	s.env.OnActivity(s.activities.PublishGameEvent, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			params := args.Get(1).(activities.PublishGameEventParams)

			if params.EventType == gameevents.TypeArtistConfirmed {
				var payload gameevents.ArtistConfirmedPayload
				raw := params.EventPayload.(map[string]interface{})
				b, err := json.Marshal(raw)
				s.Require().NoError(err)
				s.Require().NoError(json.Unmarshal(b, &payload))

				artistID = payload.ArtistID
			}

			if params.EventType == gameevents.TypeBeginDrawing {
				s.env.SignalWorkflow(SignalArtistDisconnected, ArtistDisconnectedSignal{
					GameID:       gameID,
					CurrentRound: DefaultRoundNumber,
					ArtistID:     artistID,
				})
			}

			if params.EventType == gameevents.TypeArtistDisconnected {
				sentArtistDisconnectedEvent = true

				var payload gameevents.ArtistDisconnectedPayload
				raw := params.EventPayload.(map[string]interface{})
				b, err := json.Marshal(raw)
				s.Require().NoError(err)
				s.Require().NoError(json.Unmarshal(b, &payload))

				s.Equal(artistID, payload.ArtistID)
				s.Equal(uint8(DefaultRoundNumber), payload.CurrentRound)
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
			if params.EventType == gameevents.TypeEndDrawing {
				val, err := s.env.QueryWorkflow(QueryGetGameState)
				s.NoError(err)

				var state types.GameStateView
				s.NoError(val.Get(&state))
				capturedState = &state

				s.env.SignalWorkflow(SignalTerminateGame, TerminateGameSignal{
					GameID: gameID,
					Reason: "test",
				})
			}

			return &activities.PublishGameEventResult{
				MessageID: "some-message-id",
			}, nil
		})

	s.env.SetStartWorkflowOptions(client.StartWorkflowOptions{
		ID: serverID.String(),
	})
	s.env.ExecuteWorkflow(GameWorkflow, input)

	s.True(sentArtistDisconnectedEvent)
	s.Equal(gameID, capturedState.GameID)
	s.Equal(types.PhaseInRound, capturedState.Phase)
	s.Equal(types.SubPhaseDrawing, capturedState.SubPhase)
	s.NotEmpty(capturedState.CurrentArtist)
	s.NotEmpty(capturedState.CurrentWord)
	s.Len(capturedState.Players, 1)
}

func (s *InRoundTestSuite) TestHandlesArtistDisconnect_ArtistLeft() {
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

	var artistID uuid.UUID
	var sentArtistDisconnectedEvent bool
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

	s.env.OnActivity(s.activities.PublishGameEvent, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			params := args.Get(1).(activities.PublishGameEventParams)

			if params.EventType == gameevents.TypeArtistConfirmed {
				var payload gameevents.ArtistConfirmedPayload
				raw := params.EventPayload.(map[string]interface{})
				b, err := json.Marshal(raw)
				s.Require().NoError(err)
				s.Require().NoError(json.Unmarshal(b, &payload))

				artistID = payload.ArtistID
			}

			if params.EventType == gameevents.TypeBeginDrawing {
				s.env.SignalWorkflow(SignalPlayersLeave, PlayersLeaveSignal{
					PlayerIDs: []uuid.UUID{artistID},
				})
			}

			if params.EventType == gameevents.TypeArtistDisconnected {
				sentArtistDisconnectedEvent = true

				var payload gameevents.ArtistDisconnectedPayload
				raw := params.EventPayload.(map[string]interface{})
				b, err := json.Marshal(raw)
				s.Require().NoError(err)
				s.Require().NoError(json.Unmarshal(b, &payload))

				s.Equal(artistID, payload.ArtistID)
				s.Equal(uint8(DefaultRoundNumber), payload.CurrentRound)
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
			if params.EventType == gameevents.TypeEndDrawing {
				val, err := s.env.QueryWorkflow(QueryGetGameState)
				s.NoError(err)

				var state types.GameStateView
				s.NoError(val.Get(&state))
				capturedState = &state

				s.env.SignalWorkflow(SignalTerminateGame, TerminateGameSignal{
					GameID: gameID,
					Reason: "test",
				})
			}

			return &activities.PublishGameEventResult{
				MessageID: "some-message-id",
			}, nil
		})

	s.env.SetStartWorkflowOptions(client.StartWorkflowOptions{
		ID: serverID.String(),
	})
	s.env.ExecuteWorkflow(GameWorkflow, input)

	s.True(sentArtistDisconnectedEvent)
	s.Equal(gameID, capturedState.GameID)
	s.Equal(types.PhaseInRound, capturedState.Phase)
	s.Equal(types.SubPhaseDrawing, capturedState.SubPhase)
	s.NotEmpty(capturedState.CurrentArtist)
	s.NotEmpty(capturedState.CurrentWord)
	s.Len(capturedState.Players, 1)
}

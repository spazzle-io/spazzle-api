package workflow

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/eventbus"

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

type EndRoundTestSuite struct {
	WorkflowTestSuite
}

func TestEndRound(t *testing.T) {
	suite.Run(t, new(EndRoundTestSuite))
}

func (s *EndRoundTestSuite) TestEndRound() {
	s.env.OnActivity(s.activities.SelectRandomWord, mock.Anything, mock.Anything).
		Maybe().
		Return(&activities.SelectRandomWordResult{
			Word: gofakeit.Word(),
		}, nil)

	s.env.OnActivity(s.activities.ArchiveGame, mock.Anything, mock.Anything).
		Maybe().
		Return(&activities.ArchiveGameResult{}, nil)

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

	numPlayerRoundResultsSent := 0
	sentEventOnGameEventsStream := false
	sentEventOnDrawingUpdatesStream := false

	s.env.OnActivity(s.activities.PublishGameEvents, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			params := args.Get(1).(activities.PublishGameEventsParams)

			switch params.EventType {
			case gameevents.TypePlayerRoundResult:
				for _, event := range params.Events {
					var payload gameevents.PlayerRoundResult
					raw := event.EventPayload.(map[string]interface{})
					b, err := json.Marshal(raw)
					s.Require().NoError(err)
					s.Require().NoError(json.Unmarshal(b, &payload))
					numPlayerRoundResultsSent++
				}

			case gameevents.TypeRoundEnded:
				for _, event := range params.Events {
					if params.EventType == gameevents.TypeRoundEnded && params.StreamType == eventbus.GameEventsStreamType {
						sentEventOnGameEventsStream = true

						var payload gameevents.RoundEndedPayload
						raw := event.EventPayload.(map[string]interface{})
						b, err := json.Marshal(raw)
						s.Require().NoError(err)
						s.Require().NoError(json.Unmarshal(b, &payload))

						s.Equal(uint8(DefaultRoundNumber), payload.Round)
						s.NotEmpty(payload.ArtistID)
						s.NotEmpty(payload.Word)
						s.NotEmpty(payload.DrawingDuration)
						s.NotEmpty(payload.Results)
						s.NotEmpty(payload.TotalPot)
						s.False(payload.IsFinalRound)
					}

					if params.EventType == gameevents.TypeRoundEnded && params.StreamType == eventbus.DrawingUpdatesStreamType {
						sentEventOnDrawingUpdatesStream = true
					}
				}

			default:
				for _, event := range params.Events {
					if event.TargetClientID == uuid.Nil {
						continue
					}

					s.env.SignalWorkflow(SignalEventAck, gameevents.EventAckPayload{
						CorrelationID: event.CorrelationID,
						InstanceID:    instanceID,
						Status:        gameevents.AckStatusDelivered,
					})
				}
			}

			if params.EventType == gameevents.TypeRoundEnded {
				s.env.SignalWorkflow(SignalTerminateGame, TerminateGameSignal{
					GameID: gameID,
					Reason: "test",
				})
			}
		}).
		Return(func(ctx context.Context, params activities.PublishGameEventsParams) (*activities.PublishGameEventsResult, error) {
			return &activities.PublishGameEventsResult{}, nil
		})

	s.env.SetStartWorkflowOptions(client.StartWorkflowOptions{
		ID: serverID.String(),
	})
	s.env.ExecuteWorkflow(GameWorkflow, input)

	val, err := s.env.QueryWorkflow(QueryGetGameState)
	s.NoError(err)
	var capturedState types.GameStateView
	s.NoError(val.Get(&capturedState))

	s.True(sentEventOnGameEventsStream)
	s.True(sentEventOnDrawingUpdatesStream)
	s.Equal(2, numPlayerRoundResultsSent)
	s.Equal(gameID, capturedState.GameID)
	s.Len(capturedState.Players, 2)
}

func TestGetSortedGuesses(t *testing.T) {
	guess1 := types.CorrectGuess{
		PlayerID:  uuid.New(),
		Timestamp: time.Now().UTC(),
	}

	guess2 := types.CorrectGuess{
		PlayerID:  uuid.New(),
		Timestamp: time.Now().Add(time.Hour).UTC(),
	}

	guess3 := types.CorrectGuess{
		PlayerID:  uuid.New(),
		Timestamp: time.Now().Add(2 * time.Hour).UTC(),
	}

	state := &GameState{
		CurrentRound: DefaultRoundNumber,
		CorrectGuesses: []types.CorrectGuess{
			guess3, guess1, guess2,
		},
	}

	sortedGuesses := getSortedGuesses(state)
	require.Equal(t, []types.CorrectGuess{guess1, guess2, guess3}, sortedGuesses)
}

func TestCountParticipatingPlayers(t *testing.T) {
	artistID := uuid.New()

	players := map[uuid.UUID]*PlayerGameState{
		uuid.New(): {
			IsEjected:   false,
			IsConnected: false,
		},
		uuid.New(): {
			IsEjected: true,
		},
		uuid.New(): {
			IsEjected: false,
			PlayerID:  artistID,
		},
		uuid.New(): {
			IsEjected: false,
		},
	}

	state := &GameState{
		Players:       players,
		CurrentArtist: artistID,
	}

	participatingPlayers := countParticipatingPlayers(state)
	require.Equal(t, 2, participatingPlayers)
}

func TestProcessCorrectGuessers(t *testing.T) {
	artist := &PlayerGameState{
		PlayerID:    uuid.New(),
		IsEjected:   false,
		IsConnected: true,
		Points:      100,
		StakeLost:   "200000000000000",
	}

	player1 := &PlayerGameState{
		PlayerID:    uuid.New(),
		IsEjected:   true,
		IsConnected: true,
		Points:      10,
		StakeLost:   "100000000000000",
	}

	player2 := &PlayerGameState{
		PlayerID:    uuid.New(),
		IsEjected:   false,
		IsConnected: false,
		Points:      25,
		StakeLost:   "200000000000000",
	}

	player3 := &PlayerGameState{
		PlayerID:    uuid.New(),
		IsEjected:   false,
		IsConnected: false,
		Points:      25,
		StakeLost:   "200000000000000",
	}

	player4 := &PlayerGameState{
		PlayerID:    uuid.New(),
		IsEjected:   false,
		IsConnected: true,
		Points:      30,
		StakeLost:   "150000000000000",
	}

	player5 := &PlayerGameState{
		PlayerID:    uuid.New(),
		IsEjected:   false,
		IsConnected: true,
		Points:      30,
		StakeLost:   "150000000000000",
	}

	player6 := &PlayerGameState{
		PlayerID:    uuid.New(),
		IsEjected:   false,
		IsConnected: true,
		Points:      30,
		StakeLost:   "150000000000000",
	}

	guesses := []types.CorrectGuess{
		{
			PlayerID:  uuid.New(),
			Timestamp: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			PlayerID:  artist.PlayerID,
			Timestamp: time.Date(2026, 1, 1, 0, 0, 10, 0, time.UTC),
		},
		{
			PlayerID:  player1.PlayerID,
			Timestamp: time.Date(2026, 1, 1, 0, 0, 15, 0, time.UTC),
		},
		{
			PlayerID:  player2.PlayerID,
			Timestamp: time.Date(2026, 1, 1, 0, 0, 30, 0, time.UTC),
		},
		{
			PlayerID:  player3.PlayerID,
			Timestamp: time.Date(2026, 1, 1, 0, 0, 35, 0, time.UTC),
		},
		{
			PlayerID:  player4.PlayerID,
			Timestamp: time.Date(2026, 1, 1, 0, 0, 40, 0, time.UTC),
		},
		{
			PlayerID:  player5.PlayerID,
			Timestamp: time.Date(2026, 1, 1, 0, 0, 45, 0, time.UTC),
		},
		{
			PlayerID:  player6.PlayerID,
			Timestamp: time.Date(2026, 1, 1, 0, 0, 50, 0, time.UTC),
		},
	}

	state := &GameState{
		Players: map[uuid.UUID]*PlayerGameState{
			artist.PlayerID:  artist,
			player1.PlayerID: player1,
			player2.PlayerID: player2,
			player3.PlayerID: player3,
			player4.PlayerID: player4,
			player5.PlayerID: player5,
			player6.PlayerID: player6,
		},
		CurrentArtist:    artist.PlayerID,
		DrawingStartedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		StakePerRound:    "200000000000000",
		DrawingDuration:  time.Minute,
	}

	roundResults, lastPos, err := processCorrectGuessers(state, 5, guesses)
	require.NoError(t, err)

	expectedResults := []*gameevents.PlayerRoundResult{
		{
			PlayerID:       player2.PlayerID,
			WasArtist:      false,
			GuessTimeMs:    30000,
			Tier:           "top_20",
			RoundPosition:  1,
			RoundPoints:    50,
			RoundStakeLost: "0",
			TotalPoints:    75,
			TotalStakeLost: "200000000000000",
		},
		{
			PlayerID:       player3.PlayerID,
			WasArtist:      false,
			GuessTimeMs:    35000,
			Tier:           "top_50",
			RoundPosition:  2,
			RoundPoints:    20,
			RoundStakeLost: "0",
			TotalPoints:    45,
			TotalStakeLost: "200000000000000",
		},
		{
			PlayerID:       player4.PlayerID,
			WasArtist:      false,
			GuessTimeMs:    40000,
			Tier:           "top_80",
			RoundPosition:  3,
			RoundPoints:    8,
			RoundStakeLost: "0",
			TotalPoints:    38,
			TotalStakeLost: "150000000000000",
		},
		{
			PlayerID:       player5.PlayerID,
			WasArtist:      false,
			GuessTimeMs:    45000,
			Tier:           "top_80",
			RoundPosition:  4,
			RoundPoints:    6,
			RoundStakeLost: "0",
			TotalPoints:    36,
			TotalStakeLost: "150000000000000",
		},
		{
			PlayerID:       player6.PlayerID,
			WasArtist:      false,
			GuessTimeMs:    50000,
			Tier:           TierUnranked,
			RoundPosition:  5,
			RoundPoints:    0,
			RoundStakeLost: "200000000000000",
			TotalPoints:    30,
			TotalStakeLost: "350000000000000",
		},
	}

	require.Equal(t, 5, lastPos)
	require.Len(t, roundResults, 5)
	require.Equal(t, expectedResults, roundResults)
}

func TestProcessNonGuessers(t *testing.T) {
	artist := &PlayerGameState{
		PlayerID:    uuid.New(),
		IsEjected:   false,
		IsConnected: true,
		Points:      100,
		StakeLost:   "200000000000000",
	}

	player1 := &PlayerGameState{
		PlayerID:    uuid.New(),
		IsEjected:   true,
		IsConnected: true,
		Points:      10,
		StakeLost:   "100000000000000",
	}

	player2 := &PlayerGameState{
		PlayerID:    uuid.New(),
		IsEjected:   false,
		IsConnected: false,
		Points:      25,
		StakeLost:   "200000000000000",
	}

	player3 := &PlayerGameState{
		PlayerID:    uuid.New(),
		IsEjected:   false,
		IsConnected: true,
		Points:      15,
		StakeLost:   "200000000000000",
	}

	player4 := &PlayerGameState{
		PlayerID:    uuid.New(),
		IsEjected:   false,
		IsConnected: false,
		Points:      20,
		StakeLost:   "120000000000000",
	}

	correctGuessers := map[uuid.UUID]struct{}{
		player2.PlayerID: {},
	}

	state := &GameState{
		CurrentArtist: artist.PlayerID,
		CurrentRound:  DefaultRoundNumber,
		Players: map[uuid.UUID]*PlayerGameState{
			artist.PlayerID:  artist,
			player1.PlayerID: player1,
			player2.PlayerID: player2,
			player3.PlayerID: player3,
			player4.PlayerID: player4,
		},
		CorrectGuessers: correctGuessers,
		StakePerRound:   "200000000000000",
	}

	roundResults, err := processNonGuessers(state, 5)
	require.NoError(t, err)

	expectedResults := []*gameevents.PlayerRoundResult{
		{
			PlayerID:       player3.PlayerID,
			WasArtist:      false,
			GuessTimeMs:    -1,
			Tier:           TierUnranked,
			RoundPosition:  5,
			RoundPoints:    0,
			RoundStakeLost: "200000000000000",
			TotalPoints:    15,
			TotalStakeLost: "400000000000000",
		},
		{
			PlayerID:       player4.PlayerID,
			WasArtist:      false,
			GuessTimeMs:    -1,
			Tier:           TierUnranked,
			RoundPosition:  5,
			RoundPoints:    0,
			RoundStakeLost: "200000000000000",
			TotalPoints:    20,
			TotalStakeLost: "320000000000000",
		},
	}

	require.Len(t, roundResults, 2)
	require.ElementsMatch(t, expectedResults, roundResults)
}

func TestProcessArtist_EjectedArtist(t *testing.T) {
	artist := &PlayerGameState{
		PlayerID:    uuid.New(),
		IsEjected:   true,
		IsConnected: true,
		Points:      100,
		StakeLost:   "200000000000000",
	}

	state := &GameState{
		Players: map[uuid.UUID]*PlayerGameState{
			artist.PlayerID: artist,
		},
		CurrentArtist: artist.PlayerID,
	}

	result, err := processArtist(state, 10, 2)
	require.NoError(t, err)
	require.Nil(t, result)
}

func TestProcessArtist_ArtistDoesNotExist(t *testing.T) {
	artist := &PlayerGameState{
		PlayerID:    uuid.New(),
		IsEjected:   true,
		IsConnected: true,
		Points:      100,
		StakeLost:   "200000000000000",
	}

	state := &GameState{
		Players:       map[uuid.UUID]*PlayerGameState{},
		CurrentArtist: artist.PlayerID,
	}

	result, err := processArtist(state, 10, 2)
	require.NoError(t, err)
	require.Nil(t, result)
}

func TestProcessArtist_CorrectGuessesMade(t *testing.T) {
	artist := &PlayerGameState{
		PlayerID:    uuid.New(),
		IsEjected:   false,
		IsConnected: false,
		Points:      10,
		StakeLost:   "110000000000000",
	}

	state := &GameState{
		Players: map[uuid.UUID]*PlayerGameState{
			artist.PlayerID: artist,
		},
		CurrentArtist: artist.PlayerID,
	}

	result, err := processArtist(state, 100, 2)
	require.NoError(t, err)

	expectedResult := &gameevents.PlayerRoundResult{
		PlayerID:       artist.PlayerID,
		WasArtist:      true,
		GuessTimeMs:    -1,
		Tier:           TierArtist,
		RoundPosition:  -1,
		RoundPoints:    50,
		RoundStakeLost: "0",
		TotalPoints:    60,
		TotalStakeLost: "110000000000000",
	}

	require.Equal(t, expectedResult, result)
}

func TestProcessArtist_NoCorrectGuessesMade(t *testing.T) {
	artist := &PlayerGameState{
		PlayerID:    uuid.New(),
		IsEjected:   false,
		IsConnected: true,
		Points:      10,
		StakeLost:   "110000000000000",
	}

	state := &GameState{
		Players: map[uuid.UUID]*PlayerGameState{
			artist.PlayerID: artist,
		},
		CurrentArtist: artist.PlayerID,
		StakePerRound: "200000000000000",
	}

	result, err := processArtist(state, 0, 0)
	require.NoError(t, err)

	expectedResult := &gameevents.PlayerRoundResult{
		PlayerID:       artist.PlayerID,
		WasArtist:      true,
		GuessTimeMs:    -1,
		Tier:           TierArtist,
		RoundPosition:  -1,
		RoundPoints:    0,
		RoundStakeLost: "200000000000000",
		TotalPoints:    10,
		TotalStakeLost: "310000000000000",
	}

	require.Equal(t, expectedResult, result)
}

func TestDetermineTier_NoParticipatingPlayers(t *testing.T) {
	tier := determineTier(1, 0, 0)
	require.Equal(t, TierUnranked, tier)
}

func TestDetermineTier_OneTotalGuess(t *testing.T) {
	tier := determineTier(1, 10, 1)
	require.Equal(t, "top_20", tier)
}

func TestDetermineTier(t *testing.T) {
	tier := determineTier(4, 6, 10)
	require.Equal(t, "top_80", tier)
}

func TestDetermineTier_Unranked(t *testing.T) {
	tier := determineTier(5, 6, 10)
	require.Equal(t, TierUnranked, tier)
}

func TestUpdatePlayerStates(t *testing.T) {
	playerID := uuid.New()

	results := []*gameevents.PlayerRoundResult{
		{
			PlayerID:       playerID,
			WasArtist:      false,
			GuessTimeMs:    35000,
			Tier:           "top_50",
			RoundPosition:  2,
			RoundPoints:    20,
			RoundStakeLost: "130000000000000",
			TotalPoints:    45,
			TotalStakeLost: "200000000000000",
		},
	}

	players := map[uuid.UUID]*PlayerGameState{
		playerID: {
			PlayerID:     playerID,
			Points:       10,
			StakeLost:    "100000000000000",
			RoundsPlayed: 2,
		},
	}

	state := &GameState{
		Players: players,
		GamePot: "100000000000000",
	}

	err := updatePlayerStates(state, results)
	require.NoError(t, err)

	expectedUpdatedPlayer := &PlayerGameState{
		PlayerID:     playerID,
		Points:       45,
		StakeLost:    "200000000000000",
		RoundsPlayed: 3,
	}

	expectedGamePot := "230000000000000"

	require.Equal(t, expectedUpdatedPlayer, state.Players[playerID])
	require.Equal(t, expectedGamePot, state.GamePot)
}

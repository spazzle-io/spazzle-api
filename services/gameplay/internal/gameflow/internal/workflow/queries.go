package workflow

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow/types"
	"go.temporal.io/sdk/workflow"
)

const QueryGetGameState = "get-game-state"

func registerWorkflowQueries(ctx workflow.Context, state *GameState) error {
	if err := registerGetGameStateQuery(ctx, state); err != nil {
		return fmt.Errorf("failed to register get game state query: %w", err)
	}

	return nil
}

func registerGetGameStateQuery(ctx workflow.Context, state *GameState) error {
	return workflow.SetQueryHandler(ctx, QueryGetGameState, func() (*types.GameStateView, error) {
		activePlayers := make(map[uuid.UUID]bool)
		for _, playerID := range sortedUUIDs(state.Players) {
			player := state.Players[playerID]
			if player.IsConnected && !player.IsEjected {
				activePlayers[playerID] = true
			}
		}

		numCorrectGuesses := make(map[uint8]int)
		for _, round := range workflow.DeterministicKeys(state.CorrectGuesses) {
			correctGuesses := state.CorrectGuesses[round]
			numCorrectGuesses[round] = len(correctGuesses)
		}

		return &types.GameStateView{
			GameID:            state.GameID,
			StartedAt:         state.StartedAt,
			DrawingDuration:   state.DrawingDuration,
			EndedAt:           state.EndedAt,
			Phase:             state.Phase,
			SubPhase:          state.SubPhase,
			CurrentRound:      state.CurrentRound,
			NumRounds:         state.NumRounds,
			CurrentArtist:     state.CurrentArtist,
			CurrentWord:       state.CurrentWord,
			Players:           activePlayers,
			NumCorrectGuesses: numCorrectGuesses,
			StakePerGame:      state.StakePerGame,
		}, nil
	})
}

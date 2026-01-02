package workflow

import (
	"fmt"

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
		return &types.GameStateView{
			Phase:         state.Phase,
			SubPhase:      state.SubPhase,
			RoundNumber:   state.RoundNumber,
			CurrentArtist: state.CurrentArtist,
			CurrentWord:   state.CurrentWord,
		}, nil
	})
}

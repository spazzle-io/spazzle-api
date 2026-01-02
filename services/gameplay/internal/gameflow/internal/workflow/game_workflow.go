package workflow

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow/types"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

type GameWorkflowParams struct {
	State *GameState
	Input types.GameInput
}

func GameWorkflow(ctx workflow.Context, params GameWorkflowParams) (types.GameOutput, error) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			MaximumInterval:    time.Minute,
			BackoffCoefficient: 2,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	logger := workflow.GetLogger(ctx)
	logger.Info("started game workflow")

	state := initializeGameState(ctx, params.State, params.Input)
	err := registerWorkflowQueries(ctx, state)
	if err != nil {
		return types.GameOutput{}, fmt.Errorf("failed to register workflow queries: %w", err)
	}

	notifyCh := workflow.NewChannel(ctx)

	registerGlobalSignalHandlers(ctx, state, notifyCh, logger)

	for {
		switch state.Phase {
		case types.PhaseWaiting:
			handlePhaseWaiting(ctx, state, notifyCh, logger)

		case types.PhasePrepareRound:
			handlePhasePrepareRound(ctx, state, notifyCh, logger)

		case types.PhaseInRound:
			handlePhaseInRound(ctx, state, notifyCh, logger)

		case types.PhaseEndRound:
			// return GameWorkflowResult{}, workflow.NewContinueAsNewError(
			//	ctx,
			//	GameWorkflow,
			//	GameWorkflowParams{
			//		State: state,
			//		Input: params.Input,
			//	},
			//)
			return types.GameOutput{}, nil
		}
	}
}

func initializeGameState(ctx workflow.Context, state *GameState, input types.GameInput) *GameState {
	if state != nil {
		return state
	}

	return &GameState{
		Phase:       types.PhaseWaiting,
		SubPhase:    types.SubPhaseNone,
		RoundNumber: DefaultRoundNumber,
		StartedAt:   workflow.Now(ctx).UTC(),

		Players:              make(map[uuid.UUID]*PlayerState),
		MinNumPlayersToStart: DefaultMinNumPlayersToStart,
		NumDrawingOptions:    input.NumDrawingOptions,

		PastArtists: make(map[uuid.UUID]bool),
		PendingAcks: make(map[uuid.UUID]*PendingAck),

		GameServerInstances:             make(map[uuid.UUID]*GameServerInstanceState),
		GameServerInstancesLastPrunedAt: workflow.Now(ctx).UTC(),
		GameServerInstanceTimeout:       types.GameServerInstanceTimeout,

		PlayerReports: make(map[uuid.UUID]map[uuid.UUID]bool),
	}
}

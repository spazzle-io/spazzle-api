package workflow

import (
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

	notifyCh := workflow.NewChannel(ctx)

	registerGlobalSignalHandlers(ctx, state, notifyCh, logger)

	for {
		switch state.Phase {
		case PhaseWaiting:
			handlePhaseWaiting(ctx, state, notifyCh, logger)

		case PhasePrepareRound:
			// return GameWorkflowResult{}, workflow.NewContinueAsNewError(
			//	ctx,
			//	GameWorkflow,
			//	GameWorkflowParams{
			//		State: state,
			//		Input: params.Input,
			//	},
			//)
			handlePhasePrepareRound(ctx, state, notifyCh, logger)
			return types.GameOutput{}, nil
		}
	}
}

func initializeGameState(ctx workflow.Context, state *GameState, input types.GameInput) *GameState {
	if state != nil {
		return state
	}

	return &GameState{
		Phase:       PhaseWaiting,
		RoundNumber: DefaultRoundNumber,
		StartedAt:   workflow.Now(ctx).UTC(),

		Players:              make(map[uuid.UUID]*PlayerState),
		MinNumPlayersToStart: input.MinNumPlayersToStart,

		GameServerInstances:             make(map[uuid.UUID]*GameServerInstanceState),
		GameServerInstancesLastPrunedAt: workflow.Now(ctx).UTC(),
		GameServerInstanceTimeout:       types.GameServerInstanceTimeout,
	}
}

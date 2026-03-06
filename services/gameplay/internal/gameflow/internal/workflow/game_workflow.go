package workflow

import (
	"fmt"
	"time"

	commonUtil "github.com/spazzle-io/spazzle-api/libs/common/util"
	"go.temporal.io/sdk/log"

	"github.com/google/uuid"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow/types"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

func GameWorkflow(ctx workflow.Context, input types.GameInput) (types.GameOutput, error) {
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

	state := initializeGameState(ctx, input)
	logger = log.With(logger, "GameID", state.GameID, "Round", state.CurrentRound)

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
			handlePhaseEndRound(ctx, state, notifyCh, logger)
			logger = log.With(logger, "Round", state.CurrentRound)

		case types.PhaseEndGame:
			handlePhaseEndGame(ctx, state, notifyCh, logger)
			return types.GameOutput{}, nil
		}
	}
}

func initializeGameState(ctx workflow.Context, input types.GameInput) *GameState {
	return &GameState{
		GameID: input.GameID,

		Phase:         types.PhaseWaiting,
		SubPhase:      types.SubPhaseNone,
		NumRounds:     input.NumRounds,
		CurrentRound:  DefaultRoundNumber,
		StartedAt:     workflow.Now(ctx).UTC(),
		GamePot:       "0",
		StakePerGame:  input.StakePerGame,
		StakePerRound: commonUtil.DivBigIntString(input.StakePerGame, int64(input.NumRounds)),

		Players:              make(map[uuid.UUID]*PlayerGameState),
		MinNumPlayersToStart: DefaultMinNumPlayersToStart,
		DrawingDuration:      input.DrawingDuration,

		CorrectGuesses:  make(map[uint8][]types.CorrectGuess),
		CorrectGuessers: make(map[uint8]map[uuid.UUID]bool),

		PastArtists: make(map[uuid.UUID]bool),
		PendingAcks: make(map[uuid.UUID]*PendingAck),

		GameServerInstances:             make(map[uuid.UUID]*GameServerInstanceState),
		GameServerInstancesLastPrunedAt: workflow.Now(ctx).UTC(),

		PlayerReports:  make(map[uuid.UUID]map[uuid.UUID]bool),
		EjectedPlayers: make(map[uuid.UUID]bool),
	}
}

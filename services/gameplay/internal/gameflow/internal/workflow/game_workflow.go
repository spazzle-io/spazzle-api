package workflow

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	commonUtil "github.com/spazzle-io/spazzle-api/libs/common/util"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow/types"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const (
	DefaultRoundNumber          = 1
	DefaultMinNumPlayersToStart = 2

	phaseCooldownDuration = time.Second * 1
)

func GameWorkflow(ctx workflow.Context, input types.GameInput) (types.GameOutput, error) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout:    time.Minute,
		ScheduleToCloseTimeout: 10 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			MaximumInterval:    30 * time.Second,
			BackoffCoefficient: 2,
		},
	}

	ctx = workflow.WithActivityOptions(ctx, ao)

	logger := workflow.GetLogger(ctx)

	state, err := initializeGameState(ctx, input)
	if err != nil {
		return types.GameOutput{}, fmt.Errorf("failed to initialize game state: %w", err)
	}

	state.baseLogger = logger
	logger.Info("started game workflow")

	err = registerWorkflowQueries(ctx, state)
	if err != nil {
		return types.GameOutput{}, fmt.Errorf("failed to register workflow queries: %w", err)
	}

	notifyCh := workflow.NewChannel(ctx)

	registerGlobalSignalHandlers(ctx, state, notifyCh)

	for {
		switch state.Phase {
		case types.PhaseWaiting:
			handlePhaseWaiting(ctx, state, notifyCh)

		case types.PhasePrepareRound:
			handlePhasePrepareRound(ctx, state, notifyCh)

		case types.PhaseInRound:
			handlePhaseInRound(ctx, state, notifyCh)

		case types.PhaseEndRound:
			handlePhaseEndRound(ctx, state, notifyCh)

		case types.PhaseEndGame:
			handlePhaseEndGame(ctx, state, notifyCh)
			return types.GameOutput{}, nil
		}
	}
}

func initializeGameState(ctx workflow.Context, input types.GameInput) (*GameState, error) {
	if input.NumRounds < 1 {
		return nil, nonRetryableErr(ErrTypeInvalidInput, "invalid number of rounds", nil)
	}

	stakePerGame, err := commonUtil.NewNonNegativeWei(input.StakePerGame)
	if err != nil {
		return nil, nonRetryableErr(ErrTypeInvalidInput, "invalid stake per game", err)
	}

	stakePerRound, err := stakePerGame.Div(int64(input.NumRounds))
	if err != nil {
		return nil, nonRetryableErr(ErrTypeInvalidState, "failed to determine stake per round", err)
	}

	return &GameState{
		GameID: input.GameID,

		Phase:         types.PhaseWaiting,
		SubPhase:      types.SubPhaseNone,
		NumRounds:     input.NumRounds,
		CurrentRound:  DefaultRoundNumber,
		StartedAt:     workflow.Now(ctx).UTC(),
		GamePot:       "0",
		StakePerGame:  input.StakePerGame,
		StakePerRound: stakePerRound.String(),

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
	}, nil
}

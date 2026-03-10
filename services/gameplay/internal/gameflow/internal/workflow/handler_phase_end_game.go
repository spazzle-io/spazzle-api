package workflow

import (
	"fmt"
	"sort"

	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow/internal/activities"

	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameevents"
	"go.temporal.io/sdk/log"
	"go.temporal.io/sdk/workflow"
)

func handlePhaseEndGame(ctx workflow.Context, state *GameState, notifyCh workflow.Channel, logger log.Logger) {
	logger.Info("entering end-game phase")

	for {
		err := endGame(ctx, state, notifyCh, logger)
		if err == nil {
			break
		}

		logger.Warn("error occurred in the end-game phase", "error", err)

		if err = workflow.Sleep(ctx, phaseCooldownDuration); err != nil {
			logger.Warn("failed to cooldown after end-game phase attempt", "error", err)
		}
	}
}

func endGame(ctx workflow.Context, state *GameState, notifyCh workflow.Channel, logger log.Logger) error {
	results := getFinalPlayerResults(state)

	state.EndedAt = workflow.Now(ctx).UTC()

	gameResult := gameevents.GameEndedPayload{
		TotalRounds: state.CurrentRound,
		TotalPot:    state.GamePot,
		Results:     results,
	}
	_, err := sendGameEvent(ctx, state, notifyCh, gameevents.TypeGameEnded, gameResult, nil)
	if err != nil {
		return fmt.Errorf("failed to send game ended event: %w", err)
	}

	var a *activities.Activities
	err = workflow.ExecuteActivity(ctx, a.ArchiveGame, activities.ArchiveGameParams{
		ServerID:      getGameServerID(ctx),
		GameID:        state.GameID,
		GameStake:     state.StakePerGame,
		GameStartedAt: state.StartedAt,
		GameEndedAt:   state.EndedAt,
	}).Get(ctx, nil)
	if err != nil {
		logger.Error("failed to execute archive game activity", "error", err)
	}

	logger.Info("game ended")
	return nil
}

func getFinalPlayerResults(state *GameState) []*gameevents.PlayerFinalResult {
	finalResults := make([]*gameevents.PlayerFinalResult, 0, len(state.Players))
	for _, playerID := range sortedUUIDs(state.Players) {
		player := state.Players[playerID]

		finalResults = append(finalResults, &gameevents.PlayerFinalResult{
			PlayerID:       playerID,
			TotalPoints:    player.Points,
			TotalStakeLost: player.StakeLost,
			RoundsPlayed:   player.RoundsPlayed,
			IsEjected:      player.IsEjected,
		})
	}

	sort.Slice(finalResults, func(i, j int) bool {
		return finalResults[i].TotalPoints > finalResults[j].TotalPoints
	})

	currentPosition := 1
	for i := range finalResults {
		if i > 0 && finalResults[i].TotalPoints < finalResults[i-1].TotalPoints {
			currentPosition = i + 1
		}
		finalResults[i].Position = currentPosition
	}

	finalResults = CalculateProvisionalPayouts(state, finalResults)

	return finalResults
}

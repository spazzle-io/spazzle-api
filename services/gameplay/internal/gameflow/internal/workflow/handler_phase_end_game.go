package workflow

import (
	"fmt"
	"sort"

	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow/internal/activities"

	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameevents"
	"go.temporal.io/sdk/workflow"
)

func handlePhaseEndGame(ctx workflow.Context, state *GameState, notifyCh workflow.Channel) {
	state.Logger().Info("entering end-game phase")

	for {
		err := endGame(ctx, state, notifyCh)
		if err == nil {
			break
		}

		state.Logger().Warn("error occurred in the end-game phase", "error", err)

		if err = workflow.Sleep(ctx, phaseCooldownDuration); err != nil {
			state.Logger().Warn("failed to cooldown after end-game phase attempt", "error", err)
		}
	}
}

func endGame(ctx workflow.Context, state *GameState, notifyCh workflow.Channel) error {
	results := getFinalPlayerResults(state)

	state.EndedAt = workflow.Now(ctx).UTC()

	// TODO: gameResult should only broadcast the top 10 players and not all players as this will scale linearly with
	// number of players and exceed our 4KB write limit on wss connections.
	// Proposed impl: gameResult.Results to be set to results[:min(10, len(results))]

	// TODO: given the change to only broadcast the top 10 players, use the new publish game event activity to send
	// an end game message to each player in one batch.

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
		state.Logger().Error("failed to execute archive game activity", "error", err)
	}

	state.Logger().Info("game ended")
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

package workflow

import (
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow/types"
	"go.temporal.io/sdk/log"
	"go.temporal.io/sdk/workflow"
)

func handlePhaseWaiting(ctx workflow.Context, state *GameState, notifyCh workflow.Channel, logger log.Logger) {
	logger.Info("entering waiting phase")

	for {
		if hasEnoughPlayers(state) {
			logger.Info("enough players are present in workflow to proceed")
			state.Phase = types.PhasePrepareRound
			break
		}

		if state.IsTerminated {
			state.Phase = types.PhaseEndRound
			break
		}

		workflow.NewSelector(ctx).
			AddReceive(notifyCh, func(c workflow.ReceiveChannel, more bool) {
				var tmp struct{}
				c.Receive(ctx, &tmp)
			}).
			Select(ctx)
	}
}

func hasEnoughPlayers(state *GameState) bool {
	var activePlayers int
	for _, playerID := range sortedUUIDs(state.Players) {
		player := state.Players[playerID]
		if player.IsConnected && !player.IsEjected {
			activePlayers++
		}
	}

	return activePlayers >= int(state.MinNumPlayersToStart)
}

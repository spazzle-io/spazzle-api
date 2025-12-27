package workflow

import (
	"fmt"

	"go.temporal.io/sdk/log"
	"go.temporal.io/sdk/workflow"
)

func handlePhaseWaiting(ctx workflow.Context, state *GameState, notifyCh workflow.Channel, logger log.Logger) {
	logger.Info(
		fmt.Sprintf("entering %s phase", PhaseWaiting),
		"min_num_players_to_start", state.MinNumPlayersToStart,
		"current_num_players", len(state.Players),
	)

	for {
		if hasEnoughPlayers(state) {
			logger.Info(
				"enough players are present in workflow to proceed",
				"current_num_players", len(state.Players),
			)
			break
		}

		workflow.NewSelector(ctx).
			AddReceive(notifyCh, func(c workflow.ReceiveChannel, more bool) {
				var tmp struct{}
				c.Receive(ctx, &tmp)
			}).
			Select(ctx)
	}

	state.Phase = PhasePrepareRound
}

func hasEnoughPlayers(state *GameState) bool {
	return len(state.Players) >= int(state.MinNumPlayersToStart)
}

package workflow

import (
	"fmt"

	"go.temporal.io/sdk/log"
	"go.temporal.io/sdk/workflow"
)

func handlePhasePrepareRound(ctx workflow.Context, state *GameState, notifyCh workflow.Channel, logger log.Logger) {
	logger.Info(fmt.Sprintf("entering %s phase", PhasePrepareRound))
}

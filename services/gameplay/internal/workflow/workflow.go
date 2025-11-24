package workflow

import (
	"github.com/rs/zerolog/log"
	"go.temporal.io/sdk/workflow"
)

const gameServerWorkflowTaskQueueName = "game-server-workflow"

type gameServerWorkflowParams struct{}

type gameServerWorkflowResult struct{}

func gameServerWorkflow(ctx workflow.Context, params gameServerWorkflowParams) (gameServerWorkflowResult, error) {
	log.Info().Msg("workflow ran successfully")
	return gameServerWorkflowResult{}, nil
}

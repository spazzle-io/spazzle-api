package workflow

import (
	"github.com/rs/zerolog/log"
	"go.temporal.io/sdk/workflow"
)

const GameWorkflowTaskQueueName = "game-workflow"

type GameWorkflowParams struct{}

type GameWorkflowResult struct{}

func GameWorkflow(ctx workflow.Context, params GameWorkflowParams) (GameWorkflowResult, error) {
	log.Info().Msg("workflow ran successfully")
	return GameWorkflowResult{}, nil
}

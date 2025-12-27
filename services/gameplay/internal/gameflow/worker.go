package gameflow

import (
	"github.com/rs/zerolog/log"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow/internal/activities"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow/internal/workflow"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow/types"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/util"
	temporalclient "go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

func StartWorker(config util.Config) worker.Worker {
	log.Info().Str("task_queue", types.GameWorkflowTaskQueue).Msg("starting gameFlow worker")

	opts := getTemporalClientOpts(config)
	c, err := temporalclient.Dial(opts)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to temporal")
		return nil
	}

	w := worker.New(c, types.GameWorkflowTaskQueue, worker.Options{})

	w.RegisterWorkflow(workflow.GameWorkflow)
	w.RegisterActivity(&activities.Activities{})

	go func() {
		if err := w.Run(nil); err != nil {
			log.Fatal().Err(err).Msg("gameFlow worker fatal error")
		}
	}()

	return w
}

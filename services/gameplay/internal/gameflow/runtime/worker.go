package runtime

import (
	"context"

	"github.com/rs/zerolog/log"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow/activities"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow/workflow"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/util"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

func StartWorker(ctx context.Context, config util.Config) error {
	log.Info().Str("task_queue", workflow.GameWorkflowTaskQueueName).Msg("starting temporal worker")

	opts := getTemporalClientOpts(config)
	c, err := client.Dial(opts)
	if err != nil {
		return err
	}
	defer c.Close()

	w := worker.New(c, workflow.GameWorkflowTaskQueueName, worker.Options{})

	w.RegisterWorkflow(workflow.GameWorkflow)
	w.RegisterActivity(&activities.Activities{})

	stopCh := make(chan interface{})
	go func() {
		<-ctx.Done()
		log.Info().Msg("shutting down temporal worker")
		close(stopCh)
	}()

	return w.Run(stopCh)
}

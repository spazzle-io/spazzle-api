package workflow

import (
	"context"

	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/util"

	"github.com/rs/zerolog/log"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

func StartTemporalWorker(ctx context.Context, config util.Config) error {
	log.Info().Str("task_queue", gameServerWorkflowTaskQueueName).Msg("starting temporal worker")

	opts := getTemporalClientOpts(config)
	c, err := client.Dial(opts)
	if err != nil {
		return err
	}
	defer c.Close()

	w := worker.New(c, gameServerWorkflowTaskQueueName, worker.Options{})

	w.RegisterWorkflow(gameServerWorkflow)
	w.RegisterActivity(&activities{})

	stopCh := make(chan interface{})
	go func() {
		<-ctx.Done()
		log.Info().Msg("shutting down temporal worker")
		close(stopCh)
	}()

	return w.Run(stopCh)
}

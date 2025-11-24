package workflow

import (
	"context"

	"github.com/rs/zerolog/log"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

func StartTemporalWorker(ctx context.Context) error {
	log.Info().Str("task_queue", gameServerWorkflowTaskQueueName).Msg("starting temporal worker")

	c, err := client.Dial(client.Options{
		Logger: NewTemporalLogger(log.Logger),
	})
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

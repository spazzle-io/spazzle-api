package gameflow

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"
	db "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/sqlc"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/eventbus"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow/internal/activities"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow/internal/workflow"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow/types"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/util"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/wordstore"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/worker"
	temporalclient "go.temporal.io/sdk/client"
	temporalworker "go.temporal.io/sdk/worker"
)

type Worker struct {
	Config          util.Config
	Store           db.Store
	Bus             eventbus.EventBus
	WordStore       wordstore.Store
	TaskDistributor worker.TaskDistributor
}

func (w *Worker) Run(ctx context.Context) error {
	opts := getTemporalClientOpts(w.Config)
	c, err := temporalclient.Dial(opts)
	if err != nil {
		return fmt.Errorf("failed to connect to temporal: %w", err)
	}

	wk := temporalworker.New(c, types.GameWorkflowTaskQueue, temporalworker.Options{})

	wk.RegisterWorkflow(workflow.GameWorkflow)
	wk.RegisterActivity(&activities.Activities{
		Store:           w.Store,
		Bus:             w.Bus,
		WordStore:       w.WordStore,
		TaskDistributor: w.TaskDistributor,
	})

	interruptCh := make(chan interface{})
	go func() {
		<-ctx.Done()
		close(interruptCh)
		log.Info().Msg("gameFlow worker stopped")
	}()

	log.Info().
		Str("task_queue", types.GameWorkflowTaskQueue).
		Msg("gameFlow worker started")

	return wk.Run(interruptCh)
}

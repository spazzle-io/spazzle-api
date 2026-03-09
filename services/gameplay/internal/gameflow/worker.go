package gameflow

import (
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
	worker temporalworker.Worker

	Config          util.Config
	Store           db.Store
	Bus             eventbus.EventBus
	WordStore       wordstore.Store
	TaskDistributor worker.TaskDistributor
}

func (w *Worker) Start() {
	log.Info().Str("task_queue", types.GameWorkflowTaskQueue).Msg("starting gameFlow worker")

	opts := getTemporalClientOpts(w.Config)
	c, err := temporalclient.Dial(opts)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to temporal")
		return
	}

	wk := temporalworker.New(c, types.GameWorkflowTaskQueue, temporalworker.Options{})

	wk.RegisterWorkflow(workflow.GameWorkflow)
	wk.RegisterActivity(&activities.Activities{
		Store:           w.Store,
		Bus:             w.Bus,
		WordStore:       w.WordStore,
		TaskDistributor: w.TaskDistributor,
	})

	go func() {
		if err := wk.Run(nil); err != nil {
			log.Fatal().Err(err).Msg("gameFlow worker fatal error")
		}
	}()

	w.worker = wk
}

func (w *Worker) Stop() {
	w.worker.Stop()
}

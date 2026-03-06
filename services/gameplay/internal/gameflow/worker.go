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
	temporalclient "go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

type Worker struct {
	worker worker.Worker

	Config    util.Config
	Store     db.Store
	Bus       eventbus.EventBus
	WordStore wordstore.Store
}

func (w *Worker) Start() {
	log.Info().Str("task_queue", types.GameWorkflowTaskQueue).Msg("starting gameFlow worker")

	opts := getTemporalClientOpts(w.Config)
	c, err := temporalclient.Dial(opts)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to temporal")
		return
	}

	wk := worker.New(c, types.GameWorkflowTaskQueue, worker.Options{})

	wk.RegisterWorkflow(workflow.GameWorkflow)
	wk.RegisterActivity(&activities.Activities{
		Store:     w.Store,
		Bus:       w.Bus,
		WordStore: w.WordStore,
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

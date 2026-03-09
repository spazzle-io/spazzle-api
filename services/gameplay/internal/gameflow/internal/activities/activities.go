package activities

import (
	db "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/sqlc"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/eventbus"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/wordstore"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/worker"
)

type Activities struct {
	Store           db.Store
	Bus             eventbus.EventBus
	WordStore       wordstore.Store
	TaskDistributor worker.TaskDistributor
}

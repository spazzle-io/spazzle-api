package activities

import (
	db "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/sqlc"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/eventbus"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/wordstore"
)

type Activities struct {
	Store     db.Store
	Bus       eventbus.EventBus
	WordStore wordstore.Store
}

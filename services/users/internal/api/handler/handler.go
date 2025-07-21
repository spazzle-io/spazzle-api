package handler

import (
	commonCache "github.com/spazzle-io/spazzle-api/libs/common/cache"
	commonMiddleware "github.com/spazzle-io/spazzle-api/libs/common/middleware"
	db "github.com/spazzle-io/spazzle-api/services/users/internal/db/sqlc"
	"github.com/spazzle-io/spazzle-api/services/users/internal/util"
	"github.com/spazzle-io/spazzle-api/services/users/internal/worker"
)

type Handler struct {
	config          util.Config
	store           db.Store
	cache           commonCache.Cache
	taskDistributor worker.TaskDistributor
}

func New(config util.Config, store db.Store, cache commonCache.Cache, taskDistributor worker.TaskDistributor) *Handler {
	return &Handler{
		config:          config,
		store:           store,
		cache:           cache,
		taskDistributor: taskDistributor,
	}
}

func (h *Handler) RateLimits() map[string]commonMiddleware.Rate {
	return map[string]commonMiddleware.Rate{}
}

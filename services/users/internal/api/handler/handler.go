package handler

import (
	commonCache "github.com/spazzle-io/spazzle-api/libs/common/cache"
	commonMiddleware "github.com/spazzle-io/spazzle-api/libs/common/middleware"
	db "github.com/spazzle-io/spazzle-api/services/users/internal/db/sqlc"
	"github.com/spazzle-io/spazzle-api/services/users/internal/util"
)

type Handler struct {
	config util.Config
	store  db.Store
	cache  commonCache.Cache
}

func New(config util.Config, store db.Store, cache commonCache.Cache) *Handler {
	return &Handler{
		config: config,
		store:  store,
		cache:  cache,
	}
}

func (h *Handler) RateLimits() map[string]commonMiddleware.Rate {
	return map[string]commonMiddleware.Rate{}
}

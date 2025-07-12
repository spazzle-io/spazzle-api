package handler

import (
	commonCache "github.com/spazzle-io/spazzle-api/libs/common/cache"
	pb "github.com/spazzle-io/spazzle-api/services/proto/auth/auth/v1"

	commonMiddleware "github.com/spazzle-io/spazzle-api/libs/common/middleware"
	db "github.com/spazzle-io/spazzle-api/services/auth/internal/db/sqlc"
	"github.com/spazzle-io/spazzle-api/services/auth/internal/token"
	"github.com/spazzle-io/spazzle-api/services/auth/internal/util"
)

type Handler struct {
	pb.UnimplementedAuthServiceServer

	config     util.Config
	store      db.Store
	cache      commonCache.Cache
	tokenMaker token.Maker
}

func New(config util.Config, store db.Store, cache commonCache.Cache, tokenMaker token.Maker) *Handler {
	return &Handler{
		config:     config,
		store:      store,
		cache:      cache,
		tokenMaker: tokenMaker,
	}
}

func (h *Handler) RateLimits() map[string]commonMiddleware.Rate {
	return map[string]commonMiddleware.Rate{}
}

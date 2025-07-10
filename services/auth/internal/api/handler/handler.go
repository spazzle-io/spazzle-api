package handler

import (
	"time"

	commonCache "github.com/spazzle-io/spazzle-api/libs/common/cache"

	commonMiddleware "github.com/spazzle-io/spazzle-api/libs/common/middleware"
	db "github.com/spazzle-io/spazzle-api/services/auth/internal/db/sqlc"
	"github.com/spazzle-io/spazzle-api/services/auth/internal/token"
	"github.com/spazzle-io/spazzle-api/services/auth/internal/util"
	pb "github.com/spazzle-io/spazzle-api/services/proto/auth"
)

type Handler struct {
	pb.UnimplementedAuthServer

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
	return map[string]commonMiddleware.Rate{
		"/pb.Auth/Hello": {Aliases: []string{"GET:/auth/hello"}, Limit: 10, Period: time.Hour, Identifier: "Hello"},
	}
}

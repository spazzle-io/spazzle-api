package server

import (
	commonCache "github.com/spazzle-io/spazzle-api/libs/common/cache"
	commonMiddleware "github.com/spazzle-io/spazzle-api/libs/common/middleware"
	db "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/sqlc"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/services"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/util"
	pb "github.com/spazzle-io/spazzle-api/services/proto/gameplay/gameplay/v1"
)

type Handler struct {
	pb.UnimplementedServerServiceServer

	config      util.Config
	store       db.Store
	cache       commonCache.Cache
	authService services.AuthGrpcService
}

func New(config util.Config, store db.Store, cache commonCache.Cache, authService services.AuthGrpcService) *Handler {
	return &Handler{
		config:      config,
		store:       store,
		cache:       cache,
		authService: authService,
	}
}

func (h *Handler) RateLimits() map[string]commonMiddleware.Rate {
	return map[string]commonMiddleware.Rate{}
}

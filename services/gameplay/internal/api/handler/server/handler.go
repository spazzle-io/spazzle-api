package server

import (
	"time"

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
	return map[string]commonMiddleware.Rate{
		"/gameplay.v1.ServerService/CreateServer":    {Aliases: []string{"POST:/servers"}, Limit: 30, Period: time.Minute, Identifier: "CreateServer"},
		"/gameplay.v1.ServerService/GetServer":       {Aliases: []string{"GET:/servers/{uuid}"}, Limit: 120, Period: time.Minute, Identifier: "GetServer"},
		"/gameplay.v1.ServerService/GetServerByName": {Aliases: []string{"GET:/servers/by-name/{string}"}, Limit: 120, Period: time.Minute, Identifier: "GetServerByName"},
		"/gameplay.v1.ServerService/ListServers":     {Aliases: []string{"GET:/servers"}, Limit: 120, Period: time.Minute, Identifier: "ListServers"},
	}
}

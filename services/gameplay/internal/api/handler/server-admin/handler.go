package server_admin

import (
	"time"

	commonCache "github.com/spazzle-io/spazzle-api/libs/common/cache"
	commonMiddleware "github.com/spazzle-io/spazzle-api/libs/common/middleware"
	db "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/sqlc"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/services"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/util"
	pb "github.com/spazzle-io/spazzle-api/services/proto/gameplay/gameplay/v1"
)

type HandlerConfig struct {
	Config      util.Config
	Store       db.Store
	Cache       commonCache.Cache
	AuthService services.AuthGrpcService
}

type Handler struct {
	pb.UnimplementedServerAdminServiceServer

	config      util.Config
	store       db.Store
	cache       commonCache.Cache
	authService services.AuthGrpcService
}

func New(cfg *HandlerConfig) *Handler {
	return &Handler{
		config:      cfg.Config,
		store:       cfg.Store,
		cache:       cfg.Cache,
		authService: cfg.AuthService,
	}
}

func (h *Handler) RateLimits() map[string]commonMiddleware.Rate {
	return map[string]commonMiddleware.Rate{
		"/gameplay.v1.ServerAdminService/AddServerAdmin":    {Aliases: []string{"POST:/servers/{uuid}/admins"}, Limit: 30, Period: time.Minute, Identifier: "AddServerAdmin"},
		"/gameplay.v1.ServerAdminService/RemoveServerAdmin": {Aliases: []string{"DELETE:/servers/{uuid}/admins/{uuid}"}, Limit: 30, Period: time.Minute, Identifier: "RemoveServerAdmin"},
		"/gameplay.v1.ServerAdminService/ListServerAdmins":  {Aliases: []string{"GET:/servers/{uuid}/admins"}, Limit: 120, Period: time.Minute, Identifier: "ListServerAdmins"},
	}
}

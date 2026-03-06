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

type HandlerConfig struct {
	Config      util.Config
	Store       db.Store
	Cache       commonCache.Cache
	AuthService services.AuthGrpcService
}

type Handler struct {
	pb.UnimplementedServerServiceServer

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
		"/gameplay.v1.ServerService/CreateServer":             {Aliases: []string{"POST:/servers"}, Limit: 30, Period: time.Minute, Identifier: "CreateServer"},
		"/gameplay.v1.ServerService/GetServer":                {Aliases: []string{"GET:/servers/{uuid}"}, Limit: 120, Period: time.Minute, Identifier: "GetServer"},
		"/gameplay.v1.ServerService/GetServerByName":          {Aliases: []string{"GET:/servers/by-name/{string}"}, Limit: 120, Period: time.Minute, Identifier: "GetServerByName"},
		"/gameplay.v1.ServerService/ListServers":              {Aliases: []string{"GET:/servers"}, Limit: 120, Period: time.Minute, Identifier: "ListServers"},
		"/gameplay.v1.ServerService/ListUserServers":          {Aliases: []string{"GET:/servers/by-user/{uuid}"}, Limit: 120, Period: time.Minute, Identifier: "ListUserServers"},
		"/gameplay.v1.ServerService/GetUserServerPermissions": {Aliases: []string{"GET:/servers/{uuid}/permissions/{uuid}"}, Limit: 120, Period: time.Minute, Identifier: "GetUserServerPermissions"},
		"/gameplay.v1.ServerService/UpdateServer":             {Aliases: []string{"PATCH:/servers/{uuid}"}, Limit: 30, Period: time.Minute, Identifier: "UpdateServer"},
		"/gameplay.v1.ServerService/JoinServer":               {Aliases: []string{"POST:/servers/{uuid}/join"}, Limit: 120, Period: time.Minute, Identifier: "JoinServer"},
		"/gameplay.v1.ServerService/ArchiveServer":            {Aliases: []string{"POST:/servers/{uuid}/archive"}, Limit: 10, Period: time.Minute, Identifier: "ArchiveServer"},
	}
}

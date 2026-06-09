package server

import (
	"time"

	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/infra"

	commonMiddleware "github.com/spazzle-io/spazzle-api/libs/common/middleware"
	pb "github.com/spazzle-io/spazzle-api/services/proto/gameplay/gameplay/v1"
)

type Handler struct {
	pb.UnimplementedServerServiceServer

	*infra.Resources
}

func New(res *infra.Resources) *Handler {
	return &Handler{
		Resources: res,
	}
}

func (h *Handler) RateLimits() map[string]commonMiddleware.Rate {
	return map[string]commonMiddleware.Rate{
		"/gameplay.v1.ServerService/CreateServer":             {Aliases: []string{"POST:/servers"}, Limit: 10, Period: time.Hour, Identifier: "CreateServer"},
		"/gameplay.v1.ServerService/GetServer":                {Aliases: []string{"GET:/servers/{uuid}"}, Limit: 120, Period: time.Minute, Identifier: "GetServer"},
		"/gameplay.v1.ServerService/GetServerByName":          {Aliases: []string{"GET:/servers/by-name/{string}"}, Limit: 120, Period: time.Minute, Identifier: "GetServerByName"},
		"/gameplay.v1.ServerService/ListServers":              {Aliases: []string{"GET:/servers"}, Limit: 120, Period: time.Minute, Identifier: "ListServers"},
		"/gameplay.v1.ServerService/ListUserServers":          {Aliases: []string{"GET:/servers/by-user/{uuid}"}, Limit: 120, Period: time.Minute, Identifier: "ListUserServers"},
		"/gameplay.v1.ServerService/GetUserServerPermissions": {Aliases: []string{"GET:/servers/{uuid}/permissions/{uuid}"}, Limit: 120, Period: time.Minute, Identifier: "GetUserServerPermissions"},
		"/gameplay.v1.ServerService/UpdateServer":             {Aliases: []string{"PATCH:/servers/{uuid}"}, Limit: 30, Period: time.Minute, Identifier: "UpdateServer"},
		"/gameplay.v1.ServerService/ArchiveServer":            {Aliases: []string{"POST:/servers/{uuid}/archive"}, Limit: 10, Period: time.Minute, Identifier: "ArchiveServer"},
		"/gameplay.v1.ServerService/GetServerTreasury":        {Aliases: []string{"GET:/servers/{uuid}/treasury"}, Limit: 120, Period: time.Minute, Identifier: "GetServerTreasury"},
	}
}

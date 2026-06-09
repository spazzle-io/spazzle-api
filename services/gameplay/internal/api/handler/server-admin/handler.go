package server_admin

import (
	"time"

	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/infra"

	commonMiddleware "github.com/spazzle-io/spazzle-api/libs/common/middleware"
	pb "github.com/spazzle-io/spazzle-api/services/proto/gameplay/gameplay/v1"
)

type Handler struct {
	pb.UnimplementedServerAdminServiceServer

	*infra.Resources
}

func New(res *infra.Resources) *Handler {
	return &Handler{
		Resources: res,
	}
}

func (h *Handler) RateLimits() map[string]commonMiddleware.Rate {
	return map[string]commonMiddleware.Rate{
		"/gameplay.v1.ServerAdminService/AddServerAdmin":    {Aliases: []string{"POST:/servers/{uuid}/admins"}, Limit: 30, Period: time.Minute, Identifier: "AddServerAdmin"},
		"/gameplay.v1.ServerAdminService/RemoveServerAdmin": {Aliases: []string{"DELETE:/servers/{uuid}/admins/{uuid}"}, Limit: 30, Period: time.Minute, Identifier: "RemoveServerAdmin"},
		"/gameplay.v1.ServerAdminService/ListServerAdmins":  {Aliases: []string{"GET:/servers/{uuid}/admins"}, Limit: 120, Period: time.Minute, Identifier: "ListServerAdmins"},
	}
}

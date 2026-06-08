package word

import (
	"time"

	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/infra"

	commonMiddleware "github.com/spazzle-io/spazzle-api/libs/common/middleware"
	pb "github.com/spazzle-io/spazzle-api/services/proto/gameplay/gameplay/v1"
)

type Handler struct {
	pb.UnimplementedWordServiceServer

	*infra.Resources
}

func New(res *infra.Resources) *Handler {
	return &Handler{
		Resources: res,
	}
}

func (h *Handler) RateLimits() map[string]commonMiddleware.Rate {
	return map[string]commonMiddleware.Rate{
		"/gameplay.v1.WordService/AddWords":       {Aliases: []string{"POST:/servers/{uuid}/words"}, Limit: 30, Period: time.Minute, Identifier: "AddWords"},
		"/gameplay.v1.WordService/GetRandomWords": {Aliases: []string{"GET:/servers/{uuid}/words:random"}, Limit: 60, Period: time.Minute, Identifier: "GetRandomWords"},
		"/gameplay.v1.WordService/ListWords":      {Aliases: []string{"GET:/servers/{uuid}/words"}, Limit: 60, Period: time.Minute, Identifier: "ListWords"},
		"/gameplay.v1.WordService/RemoveWords":    {Aliases: []string{"POST:/servers/{uuid}/words:remove"}, Limit: 30, Period: time.Minute, Identifier: "RemoveWords"},
		"/gameplay.v1.WordService/RemoveAllWords": {Aliases: []string{"DELETE:/servers/{uuid}/words"}, Limit: 30, Period: time.Minute, Identifier: "RemoveAllWords"},
	}
}

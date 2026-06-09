package game

import (
	"time"

	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/infra"

	commonMiddleware "github.com/spazzle-io/spazzle-api/libs/common/middleware"
	pb "github.com/spazzle-io/spazzle-api/services/proto/gameplay/gameplay/v1"
)

type Handler struct {
	pb.UnimplementedGameServiceServer

	*infra.Resources
}

func New(res *infra.Resources) *Handler {
	return &Handler{
		Resources: res,
	}
}

func (h *Handler) RateLimits() map[string]commonMiddleware.Rate {
	return map[string]commonMiddleware.Rate{
		"/gameplay.v1.GameService/ReplayGame":           {Aliases: []string{"GET:/servers/{uuid}/games/{uuid}/replay"}, Limit: 40, Period: time.Minute, Identifier: "ReplayGame"},
		"/gameplay.v1.GameService/JoinGame":             {Aliases: []string{"POST:/servers/{uuid}/games:join"}, Limit: 120, Period: time.Minute, Identifier: "JoinGame"},
		"/gameplay.v1.GameService/GetCurrentGame":       {Aliases: []string{"GET:/servers/{uuid}/games/current"}, Limit: 40, Period: time.Minute, Identifier: "GetCurrentGame"},
		"/gameplay.v1.GameService/GetGame":              {Aliases: []string{"GET:/games/{uuid}"}, Limit: 60, Period: time.Minute, Identifier: "GetGame"},
		"/gameplay.v1.GameService/GetGameLeaderboard":   {Aliases: []string{"GET:/games/{uuid}/leaderboard"}, Limit: 60, Period: time.Minute, Identifier: "GetGameLeaderboard"},
		"/gameplay.v1.GameService/GetGlobalLeaderboard": {Aliases: []string{"GET:/leaderboard"}, Limit: 60, Period: time.Minute, Identifier: "GetGlobalLeaderboard"},
		"/gameplay.v1.GameService/GetServerLeaderboard": {Aliases: []string{"GET:/servers/{uuid}/leaderboard"}, Limit: 60, Period: time.Minute, Identifier: "GetServerLeaderboard"},
		"/gameplay.v1.GameService/GetUserStats":         {Aliases: []string{"GET:/users/{uuid}/stats"}, Limit: 40, Period: time.Minute, Identifier: "GetUserStats"},
		"/gameplay.v1.GameService/ListServerGames":      {Aliases: []string{"GET:/servers/{uuid}/games"}, Limit: 40, Period: time.Minute, Identifier: "ListServerGames"},
		"/gameplay.v1.GameService/ListUserGames":        {Aliases: []string{"GET:/users/{uuid}/games"}, Limit: 40, Period: time.Minute, Identifier: "ListUserGames"},
	}
}

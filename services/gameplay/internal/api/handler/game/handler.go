package game

import (
	"time"

	commonMiddleware "github.com/spazzle-io/spazzle-api/libs/common/middleware"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/eventbus"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/services"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/util"
	pb "github.com/spazzle-io/spazzle-api/services/proto/gameplay/gameplay/v1"
)

type HandlerConfig struct {
	Config      util.Config
	Bus         eventbus.EventBus
	AuthService services.AuthGrpcService
}

type Handler struct {
	pb.UnimplementedGameServiceServer

	config      util.Config
	bus         eventbus.EventBus
	authService services.AuthGrpcService
}

func New(cfg *HandlerConfig) *Handler {
	return &Handler{
		config:      cfg.Config,
		bus:         cfg.Bus,
		authService: cfg.AuthService,
	}
}

func (h *Handler) RateLimits() map[string]commonMiddleware.Rate {
	return map[string]commonMiddleware.Rate{
		"/gameplay.v1.GameService/ReplayGame": {Aliases: []string{"GET:/servers/{uuid}/games/{uuid}/replay"}, Limit: 40, Period: time.Minute, Identifier: "ReplayGame"},
	}
}

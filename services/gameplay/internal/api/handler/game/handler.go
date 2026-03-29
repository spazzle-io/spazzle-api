package game

import (
	"time"

	commonCache "github.com/spazzle-io/spazzle-api/libs/common/cache"
	db "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/sqlc"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gamecache"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameserver"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/wordstore"

	commonMiddleware "github.com/spazzle-io/spazzle-api/libs/common/middleware"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/eventbus"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/services"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/util"
	pb "github.com/spazzle-io/spazzle-api/services/proto/gameplay/gameplay/v1"
)

type HandlerConfig struct {
	Config      util.Config
	Bus         eventbus.EventBus
	Cache       commonCache.Cache
	GameCache   *gamecache.GameCache
	Store       db.Store
	GfClient    gameflow.Client
	WordStore   wordstore.Store
	GsManager   *gameserver.Manager
	AuthService services.AuthGrpcService
}

type Handler struct {
	pb.UnimplementedGameServiceServer

	config      util.Config
	bus         eventbus.EventBus
	cache       commonCache.Cache
	gameCache   *gamecache.GameCache
	store       db.Store
	gfClient    gameflow.Client
	wordStore   wordstore.Store
	gsManager   *gameserver.Manager
	authService services.AuthGrpcService
}

func New(cfg *HandlerConfig) *Handler {
	return &Handler{
		config:      cfg.Config,
		bus:         cfg.Bus,
		cache:       cfg.Cache,
		gameCache:   cfg.GameCache,
		store:       cfg.Store,
		gfClient:    cfg.GfClient,
		wordStore:   cfg.WordStore,
		gsManager:   cfg.GsManager,
		authService: cfg.AuthService,
	}
}

func (h *Handler) RateLimits() map[string]commonMiddleware.Rate {
	return map[string]commonMiddleware.Rate{
		"/gameplay.v1.GameService/ReplayGame": {Aliases: []string{"GET:/servers/{uuid}/games/{uuid}/replay"}, Limit: 40, Period: time.Minute, Identifier: "ReplayGame"},
		"/gameplay.v1.GameService/JoinGame":   {Aliases: []string{"POST:/servers/{uuid}/games:join"}, Limit: 120, Period: time.Minute, Identifier: "JoinGame"},
	}
}

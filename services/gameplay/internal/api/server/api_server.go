package server

import (
	"fmt"
	"sync"

	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/api/handler/game"

	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/eventbus"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameserver"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/wordstore"

	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/api/handler/word"

	serveradmin "github.com/spazzle-io/spazzle-api/services/gameplay/internal/api/handler/server-admin"

	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/api/handler/server"
	db "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/sqlc"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/services"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/util"

	commonCache "github.com/spazzle-io/spazzle-api/libs/common/cache"
	commonMiddleware "github.com/spazzle-io/spazzle-api/libs/common/middleware"
	"github.com/ulule/limiter/v3"
)

type APIServerConfig struct {
	Config      util.Config
	Store       db.Store
	Cache       commonCache.Cache
	Bus         eventbus.EventBus
	GfClient    gameflow.Client
	GsManager   *gameserver.Manager
	WordStore   wordstore.Store
	AuthService services.AuthGrpcService
}

type APIServer struct {
	ServerHandler      server.Handler
	ServerAdminHandler serveradmin.Handler
	WordHandler        word.Handler
	GameHandler        game.Handler
}

var once sync.Once

func NewAPIServer(cfg *APIServerConfig) (*APIServer, error) {
	serverHandler := server.New(&server.HandlerConfig{
		Config:      cfg.Config,
		Store:       cfg.Store,
		Cache:       cfg.Cache,
		AuthService: cfg.AuthService,
	})
	serverAdminHandler := serveradmin.New(&serveradmin.HandlerConfig{
		Config:      cfg.Config,
		Store:       cfg.Store,
		Cache:       cfg.Cache,
		AuthService: cfg.AuthService,
	})
	wordHandler := word.New(&word.HandlerConfig{
		Config:      cfg.Config,
		Store:       cfg.Store,
		Cache:       cfg.Cache,
		WordStore:   cfg.WordStore,
		AuthService: cfg.AuthService,
	})
	gameHandler := game.New(&game.HandlerConfig{
		Config:      cfg.Config,
		Bus:         cfg.Bus,
		AuthService: cfg.AuthService,
	})

	err := setupRateLimiter(
		cfg.Config.ServiceName, cfg.Config.RedisConnURL,
		serverHandler.RateLimits(), serverAdminHandler.RateLimits(), wordHandler.RateLimits(), gameHandler.RateLimits(),
	)
	if err != nil {
		return nil, fmt.Errorf("cannot setup rate limiter: %w", err)
	}

	s := &APIServer{
		ServerHandler:      *serverHandler,
		ServerAdminHandler: *serverAdminHandler,
		WordHandler:        *wordHandler,
		GameHandler:        *gameHandler,
	}

	return s, nil
}

func setupRateLimiter(serviceName string, redisConnURL string, rateLimits ...map[string]commonMiddleware.Rate) error {
	var store limiter.Store
	var createLimiterRedisStoreErr, initializeLimitersErr error

	once.Do(func() {
		store, createLimiterRedisStoreErr = commonMiddleware.CreateLimiterRedisStore(serviceName, redisConnURL)
		if createLimiterRedisStoreErr == nil {
			initializeLimitersErr = commonMiddleware.InitializeLimiters(store, rateLimits...)
		}
	})

	if createLimiterRedisStoreErr != nil {
		return fmt.Errorf("could not create limiter redis client: %w", createLimiterRedisStoreErr)
	}

	if initializeLimitersErr != nil {
		return fmt.Errorf("could not initialize rate limiters: %w", initializeLimitersErr)
	}

	return nil
}

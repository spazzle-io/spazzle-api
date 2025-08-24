package server

import (
	"fmt"
	"sync"

	serveradmin "github.com/spazzle-io/spazzle-api/services/gameplay/internal/api/handler/server-admin"

	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/api/handler/server"
	db "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/sqlc"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/services"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/util"

	commonCache "github.com/spazzle-io/spazzle-api/libs/common/cache"
	commonMiddleware "github.com/spazzle-io/spazzle-api/libs/common/middleware"
	"github.com/ulule/limiter/v3"
)

type Server struct {
	ServerHandler      server.Handler
	ServerAdminHandler serveradmin.Handler
}

var once sync.Once

func New(config util.Config, store db.Store, cache commonCache.Cache) (*Server, error) {
	authService, err := services.NewAuthServiceGrpcClient(config.AuthServiceGRPCServerAddr)
	if err != nil {
		return nil, fmt.Errorf("could not create auth service gRPC client: %w", err)
	}

	serverHandler := server.New(config, store, cache, authService)
	serverAdminHandler := serveradmin.New(config, store, cache, authService)

	err = setupRateLimiter(
		config.ServiceName, config.RedisConnURL, serverHandler.RateLimits(), serverAdminHandler.RateLimits(),
	)
	if err != nil {
		return nil, fmt.Errorf("cannot setup rate limiter: %w", err)
	}

	s := &Server{
		ServerHandler:      *serverHandler,
		ServerAdminHandler: *serverAdminHandler,
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

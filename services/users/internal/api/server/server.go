package server

import (
	"fmt"

	"github.com/spazzle-io/spazzle-api/services/users/internal/services"

	commonCache "github.com/spazzle-io/spazzle-api/libs/common/cache"
	commonMiddleware "github.com/spazzle-io/spazzle-api/libs/common/middleware"
	"github.com/spazzle-io/spazzle-api/services/users/internal/api/handler"
	db "github.com/spazzle-io/spazzle-api/services/users/internal/db/sqlc"
	"github.com/spazzle-io/spazzle-api/services/users/internal/util"
)

type Server struct {
	handler.Handler
}

func New(config util.Config, store db.Store, cache commonCache.Cache) (*Server, error) {
	authService, err := services.NewAuthServiceGrpcClient(config.AuthServiceGRPCServerAddr)
	if err != nil {
		return nil, fmt.Errorf("could not create auth service gRPC client: %w", err)
	}

	h := handler.New(config, store, cache, authService)

	err = setupRateLimiter(config.ServiceName, config.RedisConnURL, h.RateLimits())
	if err != nil {
		return nil, fmt.Errorf("could not setup rate limiter: %w", err)
	}

	server := &Server{
		Handler: *h,
	}

	return server, nil
}

func setupRateLimiter(serviceName string, redisConnURL string, rateLimits map[string]commonMiddleware.Rate) error {
	store, err := commonMiddleware.CreateLimiterRedisStore(serviceName, redisConnURL)
	if err != nil {
		return fmt.Errorf("could not create rate limiter store: %w", err)
	}

	err = commonMiddleware.InitializeLimiters(store, rateLimits)
	if err != nil {
		return fmt.Errorf("could not initialize rate limiters: %w", err)
	}

	return nil
}

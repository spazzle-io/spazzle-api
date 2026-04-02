package server

import (
	"fmt"

	commonCache "github.com/spazzle-io/spazzle-api/libs/common/cache"

	commonMiddleware "github.com/spazzle-io/spazzle-api/libs/common/middleware"
	"github.com/spazzle-io/spazzle-api/services/auth/internal/api/handler"
	db "github.com/spazzle-io/spazzle-api/services/auth/internal/db/sqlc"
	"github.com/spazzle-io/spazzle-api/services/auth/internal/token"
	"github.com/spazzle-io/spazzle-api/services/auth/internal/util"
)

type Server struct {
	handler.Handler
}

func New(config util.Config, store db.Store, cache commonCache.Cache) (*Server, error) {
	tokenMaker, err := token.NewPasetoMaker(config.TokenSymmetricKey)
	if err != nil {
		return nil, fmt.Errorf("could create token maker: %w", err)
	}

	h := handler.New(config, store, cache, tokenMaker)

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

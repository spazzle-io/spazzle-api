package server

import (
	"fmt"

	"github.com/spazzle-io/spazzle-api/services/users/internal/infra"

	commonMiddleware "github.com/spazzle-io/spazzle-api/libs/common/middleware"
	"github.com/spazzle-io/spazzle-api/services/users/internal/api/handler"
)

type Server struct {
	handler.Handler
}

func New(res *infra.Resources) (*Server, error) {
	h := handler.New(res)

	err := setupRateLimiter(res.Config.ServiceName, res.Config.RedisConnURL, h.RateLimits())
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

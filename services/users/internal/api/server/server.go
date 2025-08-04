package server

import (
	"fmt"
	"sync"

	commonCache "github.com/spazzle-io/spazzle-api/libs/common/cache"
	commonMiddleware "github.com/spazzle-io/spazzle-api/libs/common/middleware"
	"github.com/spazzle-io/spazzle-api/services/users/internal/api/handler"
	db "github.com/spazzle-io/spazzle-api/services/users/internal/db/sqlc"
	"github.com/spazzle-io/spazzle-api/services/users/internal/util"
	"github.com/ulule/limiter/v3"
)

type Server struct {
	handler.Handler
}

var once sync.Once

func New(config util.Config, store db.Store, cache commonCache.Cache) (*Server, error) {
	h := handler.New(config, store, cache)

	err := setupRateLimiter(config.ServiceName, config.RedisConnURL, h.RateLimits())
	if err != nil {
		return nil, fmt.Errorf("cannot setup rate limiter: %w", err)
	}

	server := &Server{
		Handler: *h,
	}

	return server, nil
}

func setupRateLimiter(serviceName string, redisConnURL string, rateLimits map[string]commonMiddleware.Rate) error {
	var store limiter.Store
	var createLimiterRedisStoreErr, initializeLimitersErr error

	once.Do(func() {
		store, createLimiterRedisStoreErr = commonMiddleware.CreateLimiterRedisStore(serviceName, redisConnURL)
		if createLimiterRedisStoreErr == nil {
			initializeLimitersErr = commonMiddleware.InitializeLimiters(store, rateLimits)
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

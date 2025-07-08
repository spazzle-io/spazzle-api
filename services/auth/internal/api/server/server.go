package server

import (
	"fmt"
	"sync"

	commonMiddleware "github.com/spazzle-io/spazzle-api/libs/common/middleware"
	"github.com/ulule/limiter/v3"

	"github.com/spazzle-io/spazzle-api/services/auth/internal/api/handler"
	db "github.com/spazzle-io/spazzle-api/services/auth/internal/db/sqlc"
	"github.com/spazzle-io/spazzle-api/services/auth/internal/token"
	"github.com/spazzle-io/spazzle-api/services/auth/internal/util"
)

type Server struct {
	handler.Handler
}

var once sync.Once

func New(config util.Config, store db.Store) (*Server, error) {
	tokenMaker, err := token.NewPasetoMaker(config.TokenSymmetricKey)
	if err != nil {
		return nil, fmt.Errorf("cannot create token maker: %w", err)
	}

	h := handler.New(config, store, tokenMaker)

	err = setupRateLimiter(config.ServiceName, config.RedisConnURL, h.RateLimits())
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

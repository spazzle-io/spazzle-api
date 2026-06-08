package server

import (
	"fmt"

	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/infra"

	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/api/handler/game"

	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/api/handler/word"

	serveradmin "github.com/spazzle-io/spazzle-api/services/gameplay/internal/api/handler/server-admin"

	commonMiddleware "github.com/spazzle-io/spazzle-api/libs/common/middleware"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/api/handler/server"
)

type APIServer struct {
	ServerHandler      *server.Handler
	ServerAdminHandler *serveradmin.Handler
	WordHandler        *word.Handler
	GameHandler        *game.Handler
}

func NewAPIServer(res *infra.Resources) (*APIServer, error) {
	serverHandler := server.New(res)
	serverAdminHandler := serveradmin.New(res)
	wordHandler := word.New(res)
	gameHandler := game.New(res)

	err := setupRateLimiter(
		res.Config.ServiceName, res.Config.RedisConnURL,
		serverHandler.RateLimits(), serverAdminHandler.RateLimits(), wordHandler.RateLimits(), gameHandler.RateLimits(),
	)
	if err != nil {
		return nil, fmt.Errorf("could not setup rate limiter: %w", err)
	}

	s := &APIServer{
		ServerHandler:      serverHandler,
		ServerAdminHandler: serverAdminHandler,
		WordHandler:        wordHandler,
		GameHandler:        gameHandler,
	}

	return s, nil
}

func setupRateLimiter(serviceName string, redisConnURL string, rateLimits ...map[string]commonMiddleware.Rate) error {
	store, err := commonMiddleware.CreateLimiterRedisStore(serviceName, redisConnURL)
	if err != nil {
		return fmt.Errorf("could not create rate limiter store: %w", err)
	}

	err = commonMiddleware.InitializeLimiters(store, rateLimits...)
	if err != nil {
		return fmt.Errorf("could not initialize rate limiters: %w", err)
	}

	return nil
}

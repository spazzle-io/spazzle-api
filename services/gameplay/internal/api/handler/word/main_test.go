package word

import (
	commonCache "github.com/spazzle-io/spazzle-api/libs/common/cache"
	db "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/sqlc"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/services"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/util"
)

func getTestConfig() util.Config {
	return util.Config{
		ServiceName: "test",
		Environment: "development",
	}
}

func newTestHandler(store db.Store, cache commonCache.Cache, authService services.AuthGrpcService) *Handler {
	// TODO: Get rid of newTestHandler func and have tests inject their own HandlerConfig into New
	config := getTestConfig()

	handlerCfg := &HandlerConfig{
		Config:      config,
		Store:       store,
		Cache:       cache,
		AuthService: authService,
	}

	return New(handlerCfg)
}

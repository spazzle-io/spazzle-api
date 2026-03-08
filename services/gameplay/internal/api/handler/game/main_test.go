package game

import (
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/eventbus"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/services"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/util"
)

func getTestConfig() util.Config {
	return util.Config{
		ServiceName: "test",
		Environment: "development",
	}
}

func newTestHandler(bus eventbus.EventBus, authService services.AuthGrpcService) *Handler {
	config := getTestConfig()

	handlerCfg := &HandlerConfig{
		Config:      config,
		Bus:         bus,
		AuthService: authService,
	}

	return New(handlerCfg)
}

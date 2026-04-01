package game

import (
	commonCache "github.com/spazzle-io/spazzle-api/libs/common/cache"
	db "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/sqlc"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/eventbus"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gamecache"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameserver"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/services"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/util"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/wordstore"
)

func getTestConfig() util.Config {
	return util.Config{
		ServiceName: "test",
		Environment: "development",
	}
}

func newTestHandler(
	cache commonCache.Cache,
	gameCache *gamecache.GameCache,
	store db.Store,
	bus eventbus.EventBus,
	gfClient gameflow.Client,
	wordStore wordstore.Store,
	gsManager *gameserver.Manager,
	authService services.AuthGrpcService,
) *Handler {
	config := getTestConfig()

	handlerCfg := &HandlerConfig{
		Config:      config,
		Bus:         bus,
		Cache:       cache,
		GameCache:   gameCache,
		Store:       store,
		GfClient:    gfClient,
		WordStore:   wordStore,
		GsManager:   gsManager,
		AuthService: authService,
	}

	return New(handlerCfg)
}

package game

import (
	"testing"

	commonConfig "github.com/spazzle-io/spazzle-api/libs/common/config"

	mockcache "github.com/spazzle-io/spazzle-api/libs/common/cache/mock"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/api/deps"
	mockdb "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/mock"
	mockeventbus "github.com/spazzle-io/spazzle-api/services/gameplay/internal/eventbus/mock"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gamecache"
	mockgameflow "github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow/mock"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameserver"
	mockservices "github.com/spazzle-io/spazzle-api/services/gameplay/internal/services/mock"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/util"
	mockwordstore "github.com/spazzle-io/spazzle-api/services/gameplay/internal/wordstore/mock"
	"go.uber.org/mock/gomock"
)

type testDeps struct {
	ctrl        *gomock.Controller
	store       *mockdb.MockStore
	cache       *mockcache.MockCache
	gameCache   *gamecache.GameCache
	bus         *mockeventbus.MockEventBus
	gfClient    *mockgameflow.MockClient
	wordStore   *mockwordstore.MockStore
	gsManager   *gameserver.Manager
	authService *mockservices.MockAuthGrpcService
	session     *mockeventbus.MockSession
}

func getTestConfig() *util.Config {
	return &util.Config{
		AppConfig: commonConfig.AppConfig{
			ServiceName: "test",
			Environment: "development",
		},
	}
}

func newTestDeps(t *testing.T) *testDeps {
	t.Helper()

	ctrl := gomock.NewController(t)

	cache := mockcache.NewMockCache(ctrl)

	return &testDeps{
		ctrl:        ctrl,
		store:       mockdb.NewMockStore(ctrl),
		cache:       cache,
		gameCache:   gamecache.New(getTestConfig(), cache),
		bus:         mockeventbus.NewMockEventBus(ctrl),
		gfClient:    mockgameflow.NewMockClient(ctrl),
		wordStore:   mockwordstore.NewMockStore(ctrl),
		gsManager:   gameserver.NewManager(),
		authService: mockservices.NewMockAuthGrpcService(ctrl),
		session:     mockeventbus.NewMockSession(ctrl),
	}
}

func newTestHandler(d *testDeps) *Handler {
	return New(&deps.APIServerDeps{
		Config:      getTestConfig(),
		Store:       d.store,
		Cache:       d.cache,
		GameCache:   d.gameCache,
		Bus:         d.bus,
		GfClient:    d.gfClient,
		WordStore:   d.wordStore,
		GsManager:   d.gsManager,
		AuthService: d.authService,
	})
}

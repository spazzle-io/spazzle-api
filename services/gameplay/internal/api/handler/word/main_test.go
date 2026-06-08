package word

import (
	"testing"

	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/infra"

	commonConfig "github.com/spazzle-io/spazzle-api/libs/common/config"

	mockcache "github.com/spazzle-io/spazzle-api/libs/common/cache/mock"
	mockdb "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/mock"
	mockservices "github.com/spazzle-io/spazzle-api/services/gameplay/internal/services/mock"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/util"
	mockwordstore "github.com/spazzle-io/spazzle-api/services/gameplay/internal/wordstore/mock"
	"go.uber.org/mock/gomock"
)

type testDeps struct {
	ctrl        *gomock.Controller
	store       *mockdb.MockStore
	cache       *mockcache.MockCache
	authService *mockservices.MockAuthGrpcService
	wordStore   *mockwordstore.MockStore
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

	return &testDeps{
		ctrl:        ctrl,
		store:       mockdb.NewMockStore(ctrl),
		cache:       mockcache.NewMockCache(ctrl),
		authService: mockservices.NewMockAuthGrpcService(ctrl),
		wordStore:   mockwordstore.NewMockStore(ctrl),
	}
}

func newTestHandler(d *testDeps) *Handler {
	return New(&infra.Resources{
		Config:      getTestConfig(),
		Store:       d.store,
		Cache:       d.cache,
		AuthService: d.authService,
		WordStore:   d.wordStore,
	})
}

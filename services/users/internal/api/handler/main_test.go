package handler

import (
	"testing"

	mockcache "github.com/spazzle-io/spazzle-api/libs/common/cache/mock"
	mockdb "github.com/spazzle-io/spazzle-api/services/users/internal/db/mock"
	"github.com/spazzle-io/spazzle-api/services/users/internal/infra"
	mockservices "github.com/spazzle-io/spazzle-api/services/users/internal/services/mock"
	"go.uber.org/mock/gomock"

	commonConfig "github.com/spazzle-io/spazzle-api/libs/common/config"

	"github.com/spazzle-io/spazzle-api/services/users/internal/util"
	"github.com/stretchr/testify/require"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/status"
)

type testDeps struct {
	ctrl        *gomock.Controller
	store       *mockdb.MockStore
	cache       *mockcache.MockCache
	authService *mockservices.MockAuthGrpcService
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
	}
}

func newTestHandler(d *testDeps) *Handler {
	return New(&infra.Resources{
		Config:      getTestConfig(),
		Store:       d.store,
		Cache:       d.cache,
		AuthService: d.authService,
	})
}

func checkInvalidRequestParams(t *testing.T, err error, expectedFieldViolations []string) {
	var violations []string

	st, ok := status.FromError(err)
	require.True(t, ok)

	details := st.Details()

	for _, detail := range details {
		br, ok := detail.(*errdetails.BadRequest)
		require.True(t, ok)

		fieldViolations := br.FieldViolations
		for _, violation := range fieldViolations {
			violations = append(violations, violation.Field)
		}
	}

	require.ElementsMatch(t, expectedFieldViolations, violations)
}

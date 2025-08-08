package handler

import (
	"testing"

	commonCache "github.com/spazzle-io/spazzle-api/libs/common/cache"
	db "github.com/spazzle-io/spazzle-api/services/users/internal/db/sqlc"
	"github.com/spazzle-io/spazzle-api/services/users/internal/services"
	"github.com/spazzle-io/spazzle-api/services/users/internal/util"
	"github.com/stretchr/testify/require"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/status"
)

func getTestConfig() util.Config {
	return util.Config{
		ServiceName: "test",
		Environment: "development",
	}
}

func newTestHandler(store db.Store, cache commonCache.Cache, authService services.AuthGrpcService) *Handler {
	config := getTestConfig()

	return New(config, store, cache, authService)
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

package handler

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/metadata"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/status"

	"github.com/brianvoe/gofakeit/v7"
	commonCache "github.com/spazzle-io/spazzle-api/libs/common/cache"
	db "github.com/spazzle-io/spazzle-api/services/auth/internal/db/sqlc"
	"github.com/spazzle-io/spazzle-api/services/auth/internal/token"
	"github.com/spazzle-io/spazzle-api/services/auth/internal/util"
	"github.com/stretchr/testify/require"
)

func getTestConfig() util.Config {
	return util.Config{
		ServiceName:       "test",
		Environment:       "development",
		TokenSymmetricKey: gofakeit.LetterN(32),
		AllowedOrigins:    []string{"http://localhost:3000"},
	}
}

func newTestHandler(t *testing.T, store db.Store, cache commonCache.Cache) *Handler {
	config := getTestConfig()

	tokenMaker, err := token.NewPasetoMaker(config.TokenSymmetricKey)
	require.NoError(t, err)
	require.NotEmpty(t, tokenMaker)

	return New(config, store, cache, tokenMaker)
}

func newContextWithBearerToken(
	t *testing.T,
	userId uuid.UUID,
	walletAddress string,
	role token.Role,
	tokenType token.Type,
	duration time.Duration,
	tokenMaker token.Maker,
) context.Context {
	tk, _, err := tokenMaker.CreateToken(userId, walletAddress, role, tokenType, duration)
	require.NoError(t, err)
	require.NotEmpty(t, tk)

	bearerToken := fmt.Sprintf("%s %s", authorizationBearer, tk)
	md := metadata.MD{
		authorizationHeader: []string{
			bearerToken,
		},
	}

	return metadata.NewIncomingContext(context.Background(), md)
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

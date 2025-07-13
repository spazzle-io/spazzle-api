package handler

import (
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	commonCache "github.com/spazzle-io/spazzle-api/libs/common/cache"
	db "github.com/spazzle-io/spazzle-api/services/auth/internal/db/sqlc"
	"github.com/spazzle-io/spazzle-api/services/auth/internal/token"
	"github.com/spazzle-io/spazzle-api/services/auth/internal/util"
	"github.com/stretchr/testify/require"
)

func newTestHandler(t *testing.T, store db.Store, cache commonCache.Cache) *Handler {
	config := util.Config{
		Environment:       "development",
		TokenSymmetricKey: gofakeit.LetterN(32),
		AllowedOrigins:    []string{"http://localhost:3000"},
	}

	tokenMaker, err := token.NewPasetoMaker(config.TokenSymmetricKey)
	require.NoError(t, err)
	require.NotEmpty(t, tokenMaker)

	return New(config, store, cache, tokenMaker)
}

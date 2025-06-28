package db

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
	commonConfig "github.com/spazzle-io/spazzle-api/libs/common/config"
	"github.com/spazzle-io/spazzle-api/services/auth/internal/util"
)

var testStore Store

func TestMain(m *testing.M) {
	config, err := commonConfig.LoadConfig[util.Config]("../../../", ".development")
	if err != nil {
		log.Fatal().Err(err).Msg("could not load config")
	}

	connPool, err := pgxpool.New(context.Background(), config.DBSource)
	if err != nil {
		log.Fatal().Err(err).Msg("could not connect to the db")
	}

	testStore = NewStore(connPool)

	os.Exit(m.Run())
}

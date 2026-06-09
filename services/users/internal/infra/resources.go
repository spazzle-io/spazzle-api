package infra

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
	commonCache "github.com/spazzle-io/spazzle-api/libs/common/cache"
	commonConfig "github.com/spazzle-io/spazzle-api/libs/common/config"
	db "github.com/spazzle-io/spazzle-api/services/users/internal/db/sqlc"
	"github.com/spazzle-io/spazzle-api/services/users/internal/services"
	"github.com/spazzle-io/spazzle-api/services/users/internal/util"
)

type Resources struct {
	Config      *util.Config
	Store       db.Store
	ConnPool    *pgxpool.Pool
	Cache       commonCache.Cache
	AuthService services.AuthGrpcService
}

func NewResources(ctx context.Context) (res *Resources, cleanup func(), err error) {
	res = &Resources{}

	defer func() {
		if err != nil {
			res.Close()
		}
	}()

	res.Config, err = commonConfig.Load[*util.Config](".", ".development")
	if err != nil {
		return nil, nil, fmt.Errorf("could not load config: %w", err)
	}

	commonConfig.SetupLogger(res.Config.ServiceName, res.Config.Is(commonConfig.Development))

	commonConfig.RunDBMigration(res.Config.DBMigrationURL, res.Config.DBSource)

	res.ConnPool, err = pgxpool.New(ctx, res.Config.DBSource)
	if err != nil {
		return nil, nil, fmt.Errorf("could not connect to database: %w", err)
	}

	res.Store = db.NewStore(res.ConnPool)

	res.Cache, err = commonCache.NewRedisCache(res.Config.RedisConnURL)
	if err != nil {
		return nil, nil, fmt.Errorf("could not create redis cache: %w", err)
	}

	res.AuthService, err = services.NewAuthServiceGrpcClient(res.Config.AuthServiceGRPCServerAddr)
	if err != nil {
		return nil, nil, fmt.Errorf("could not create auth service client: %w", err)
	}

	return res, res.Close, nil
}

func (r *Resources) Close() {
	if r == nil {
		return
	}

	if r.AuthService != nil {
		if err := r.AuthService.Close(); err != nil {
			log.Error().Err(err).Msg("could not close auth service")
		}
	}

	if r.Cache != nil {
		if err := r.Cache.Close(); err != nil {
			log.Error().Err(err).Msg("could not close cache")
		}
	}

	if r.ConnPool != nil {
		r.ConnPool.Close()
	}
}

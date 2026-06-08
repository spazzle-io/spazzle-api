package infra

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
	commonCache "github.com/spazzle-io/spazzle-api/libs/common/cache"
	commonConfig "github.com/spazzle-io/spazzle-api/libs/common/config"
	commonStorage "github.com/spazzle-io/spazzle-api/libs/common/storage"
	db "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/sqlc"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/eventbus"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gamecache"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameserver"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/services"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/treasury"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/util"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/wordstore"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/worker"
)

type Resources struct {
	Config          *util.Config
	Store           db.Store
	ConnPool        *pgxpool.Pool
	Cache           commonCache.Cache
	GameCache       *gamecache.GameCache
	Bus             eventbus.EventBus
	GfClient        gameflow.Client
	GfWorker        *gameflow.Worker
	GsManager       *gameserver.Manager
	WordStore       wordstore.Store
	ObjectStore     commonStorage.ObjectStore
	TaskProcessor   worker.TaskProcessor
	TaskDistributor worker.TaskDistributor
	TreasuryClient  treasury.Client
	AuthService     services.AuthGrpcService
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

	res.ObjectStore, err = commonStorage.NewS3Store(
		res.Config.ObjectStoreEndpoint,
		res.Config.ObjectStoreRegion,
		res.Config.ObjectStoreAccessKey,
		res.Config.ObjectStoreSecretKey,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("could not create object store: %w", err)
	}

	res.Cache, err = commonCache.NewRedisCache(res.Config.RedisConnURL)
	if err != nil {
		return nil, nil, fmt.Errorf("could not create redis cache: %w", err)
	}

	res.Bus, err = eventbus.New(ctx, res.Config)
	if err != nil {
		return nil, nil, fmt.Errorf("could not create event bus: %w", err)
	}

	res.TreasuryClient, err = treasury.New(ctx, res.Config)
	if err != nil {
		return nil, nil, fmt.Errorf("could not create treasury client: %w", err)
	}

	redisOpt, err := asynq.ParseRedisURI(res.Config.RedisConnURL)
	if err != nil {
		return nil, nil, fmt.Errorf("could not parse redis connection url: %w", err)
	}

	res.TaskDistributor = worker.NewRedisTaskDistributor(redisOpt)
	res.TaskProcessor = worker.NewRedisTaskProcessor(redisOpt, res.Bus, res.Store, res.ObjectStore, res.TreasuryClient)

	res.AuthService, err = services.NewAuthServiceGrpcClient(res.Config.AuthServiceGRPCServerAddr)
	if err != nil {
		return nil, nil, fmt.Errorf("could not create auth service client: %w", err)
	}

	res.GameCache = gamecache.New(res.Config, res.Cache)

	res.WordStore, err = wordstore.NewDefaultStore()
	if err != nil {
		return nil, nil, fmt.Errorf("could not initialize word store: %w", err)
	}

	res.GfClient, err = gameflow.NewClient(res.Config)
	if err != nil {
		return nil, nil, fmt.Errorf("could not create gameFlow client: %w", err)
	}

	res.GfWorker = &gameflow.Worker{
		Config:          res.Config,
		Store:           res.Store,
		Bus:             res.Bus,
		WordStore:       res.WordStore,
		TaskDistributor: res.TaskDistributor,
	}

	res.GsManager = gameserver.NewManager()

	return res, res.Close, nil
}

func (r *Resources) Close() {
	if r == nil {
		return
	}

	if r.GsManager != nil {
		r.GsManager.Shutdown()
		log.Info().Msg("game server manager stopped")
	}

	if r.TaskProcessor != nil {
		r.TaskProcessor.Stop()
	}

	if r.TaskDistributor != nil {
		if err := r.TaskDistributor.Close(); err != nil {
			log.Error().Err(err).Msg("could not close task distributor")
		}
	}

	if r.GfClient != nil {
		r.GfClient.Close()
		log.Info().Msg("gameFlow client closed")
	}

	if r.TreasuryClient != nil {
		r.TreasuryClient.Close()
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

	if r.Bus != nil {
		if err := r.Bus.Close(); err != nil {
			log.Error().Err(err).Msg("could not close event bus")
		}
	}

	if r.ConnPool != nil {
		r.ConnPool.Close()
	}
}

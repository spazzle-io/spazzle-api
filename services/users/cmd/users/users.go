package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/hibiken/asynq"
	"github.com/spazzle-io/spazzle-api/services/users/internal/worker"

	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
	commonCache "github.com/spazzle-io/spazzle-api/libs/common/cache"
	commonConfig "github.com/spazzle-io/spazzle-api/libs/common/config"
	_ "github.com/spazzle-io/spazzle-api/libs/common/docs/statik"
	commonMiddleware "github.com/spazzle-io/spazzle-api/libs/common/middleware"
	commonServer "github.com/spazzle-io/spazzle-api/libs/common/server"
	"github.com/spazzle-io/spazzle-api/services/users/internal/api/server"
	db "github.com/spazzle-io/spazzle-api/services/users/internal/db/sqlc"
	"github.com/spazzle-io/spazzle-api/services/users/internal/util"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
)

var interruptSignals = []os.Signal{
	os.Interrupt,
	syscall.SIGTERM,
	syscall.SIGINT,
}

func main() {
	config, err := commonConfig.LoadConfig[util.Config](".", ".development")
	if err != nil {
		log.Fatal().Err(err).Msg("could not load config")
	}

	commonConfig.SetupLogger(config.ServiceName, config.IsDevelopmentEnvironment())

	commonConfig.RunDBMigration(config.DBMigrationURL, config.DBSource)

	ctx, stopInterruptCtx := signal.NotifyContext(context.Background(), interruptSignals...)

	connPool, err := pgxpool.New(ctx, config.DBSource)
	if err != nil {
		log.Fatal().Err(err).Msg("could not connect to database")
	}

	store := db.NewStore(connPool)

	redisOpt, err := asynq.ParseRedisURI(config.RedisConnURL)
	if err != nil {
		log.Fatal().Err(err).Msg("could not parse redis connection URL")
	}

	taskDistributor := worker.NewRedisTaskDistributor(redisOpt)

	redisCache, err := commonCache.NewRedisCache(config.RedisConnURL)
	if err != nil {
		log.Fatal().Err(err).Msg("could not create redis cache")
	}

	waitGroup, ctx := errgroup.WithContext(ctx)

	runTaskProcessor(ctx, waitGroup, config, redisOpt, redisCache)
	runGRPCServer(ctx, waitGroup, config, store, redisCache, taskDistributor)
	runGatewayServer(ctx, waitGroup, config, store, redisCache, taskDistributor)

	err = waitGroup.Wait()
	if err != nil {
		log.Fatal().Err(err).Msg("could not wait for server shutdown")
	}

	stopInterruptCtx()

	err = redisCache.Close()
	if err != nil {
		log.Fatal().Err(err).Msg("could not close redis cache")
	}
}

func runTaskProcessor(
	ctx context.Context,
	waitGroup *errgroup.Group,
	config util.Config,
	redisOpt asynq.RedisConnOpt,
	redisCache commonCache.Cache,
) {
	taskProcessor := worker.NewRedisTaskProcessor(redisOpt, config, redisCache)

	err := taskProcessor.Start()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to start task processor")
	}

	waitGroup.Go(func() error {
		<-ctx.Done()
		log.Info().Msg("shutting down task processor")

		taskProcessor.Shutdown()
		log.Info().Msg("task processor shut down")

		return nil
	})

	log.Info().Msg("started task processor")
}

func runGRPCServer(
	ctx context.Context,
	waitGroup *errgroup.Group,
	config util.Config,
	store db.Store,
	cache commonCache.Cache,
	taskDistributor worker.TaskDistributor,
) {
	_, err := server.New(config, store, cache, taskDistributor)
	if err != nil {
		log.Fatal().Err(err).Msg("could not create server")
	}

	commonServer.RunGRPCServer(
		ctx,
		waitGroup,
		config.GRPCServerAddress,
		[]commonServer.GrpcMiddlewareProvider{
			func() grpc.UnaryServerInterceptor {
				config := &commonMiddleware.AuthenticateServiceConfig{
					Cache: cache,
				}
				return config.AuthenticateServiceGrpc
			},
		},
		[]commonServer.GrpcServiceRegistrar{},
	)
}

func runGatewayServer(
	ctx context.Context,
	waitGroup *errgroup.Group,
	config util.Config,
	store db.Store,
	cache commonCache.Cache,
	taskDistributor worker.TaskDistributor,
) {
	_, err := server.New(config, store, cache, taskDistributor)
	if err != nil {
		log.Fatal().Err(err).Msg("could not create server")
	}

	commonServer.RunGatewayServer(
		ctx,
		waitGroup,
		config.HTTPServerAddress,
		config.IsDevelopmentEnvironment(),
		config.AllowedOrigins,
		[]commonServer.GatewayRouteRegistrar{},
		[]commonServer.HttpRouteRegistrar{},
		func(handler http.Handler) http.Handler {
			config := &commonMiddleware.AuthenticateServiceConfig{
				Cache: cache,
			}
			return commonMiddleware.AuthenticateServiceHTTP(handler, config)
		},
	)
}

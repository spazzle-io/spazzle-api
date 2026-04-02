package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/api/deps"

	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gamecache"

	"github.com/hibiken/asynq"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/worker"

	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/services"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/wordstore"

	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/eventbus"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow"

	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameserver"

	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/api/server/websocketserver"

	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	_ "github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
	commonCache "github.com/spazzle-io/spazzle-api/libs/common/cache"
	commonConfig "github.com/spazzle-io/spazzle-api/libs/common/config"
	_ "github.com/spazzle-io/spazzle-api/libs/common/docs/statik"
	commonMiddleware "github.com/spazzle-io/spazzle-api/libs/common/middleware"
	commonServer "github.com/spazzle-io/spazzle-api/libs/common/server"
	commonStorage "github.com/spazzle-io/spazzle-api/libs/common/storage"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/api/server"
	db "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/sqlc"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/util"
	pb "github.com/spazzle-io/spazzle-api/services/proto/gameplay/gameplay/v1"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
)

var interruptSignals = []os.Signal{
	os.Interrupt,
	syscall.SIGTERM,
}

func main() {
	ctx, stopInterruptCtx := signal.NotifyContext(context.Background(), interruptSignals...)
	waitGroup, ctx := errgroup.WithContext(ctx)

	config, err := commonConfig.LoadConfig[util.Config](".", ".development")
	if err != nil {
		log.Fatal().Err(err).Msg("could not load config")
	}

	commonConfig.SetupLogger(config.ServiceName, config.IsDevelopmentEnvironment())

	commonConfig.RunDBMigration(config.DBMigrationURL, config.DBSource)

	connPool, err := pgxpool.New(ctx, config.DBSource)
	if err != nil {
		log.Fatal().Err(err).Msg("could not connect to database")
	}

	store := db.NewStore(connPool)

	objectStore, err := commonStorage.NewS3Store(
		config.ObjectStoreEndpoint, config.ObjectStoreRegion, config.ObjectStoreAccessKey, config.ObjectStoreSecretKey,
	)
	if err != nil {
		log.Fatal().Err(err).Msg("could not create object store")
	}

	redisCache, err := commonCache.NewRedisCache(config.RedisConnURL)
	if err != nil {
		log.Fatal().Err(err).Msg("could not create redis cache")
	}

	bus, err := eventbus.New(ctx, config)
	if err != nil {
		log.Fatal().Err(err).Msg("could not create event bus")
	}

	redisOpt, err := asynq.ParseRedisURI(config.RedisConnURL)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to parse redis URI for asynq")
	}
	taskDistributor := worker.NewRedisTaskDistributor(redisOpt)
	taskProcessor := runTaskProcessor(redisOpt, store, bus, objectStore)

	authService, err := services.NewAuthServiceGrpcClient(config.AuthServiceGRPCServerAddr)
	if err != nil {
		log.Fatal().Err(err).Msg("could not create auth service client")
	}

	gfClient, gsManager, gameCache, wordStore := startGameServices(
		ctx, waitGroup, config, store, redisCache, bus, taskDistributor,
	)

	serverDeps := &deps.APIServerDeps{
		Config:          config,
		Store:           store,
		Cache:           redisCache,
		GameCache:       gameCache,
		Bus:             bus,
		WordStore:       wordStore,
		ObjectStore:     objectStore,
		TaskDistributor: taskDistributor,
		AuthService:     authService,
		GfClient:        gfClient,
		GsManager:       gsManager,
	}

	apiServer, err := server.NewAPIServer(serverDeps)
	if err != nil {
		log.Fatal().Err(err).Msg("could not create API server")
	}

	runGRPCServer(ctx, waitGroup, apiServer, serverDeps)
	runGatewayServer(ctx, waitGroup, apiServer, serverDeps)

	if err = waitGroup.Wait(); err != nil {
		log.Fatal().Err(err).Msg("service terminating due to component failure")
	}

	stopInterruptCtx()

	taskProcessor.Stop()
	if err = taskDistributor.Close(); err != nil {
		log.Error().Err(err).Msg("could not close task distributor")
	}

	if err = redisCache.Close(); err != nil {
		log.Error().Err(err).Msg("could not close redis cache")
	}

	if err = bus.Close(); err != nil {
		log.Error().Err(err).Msg("could not close event bus")
	}

	if err = authService.Close(); err != nil {
		log.Error().Err(err).Msg("could not close auth service")
	}
}

func runGRPCServer(ctx context.Context, wg *errgroup.Group, apiServer *server.APIServer, deps *deps.APIServerDeps) {
	commonServer.RunGRPCServer(
		ctx,
		wg,
		deps.Config.GRPCServerAddress,
		[]commonServer.GrpcMiddlewareProvider{
			func() grpc.UnaryServerInterceptor {
				config := &commonMiddleware.AuthenticateServiceConfig{
					Cache: deps.Cache,
				}
				return config.AuthenticateServiceGrpc
			},
		},
		[]commonServer.GrpcServiceRegistrar{
			func(grpcServer *grpc.Server) {
				pb.RegisterServerServiceServer(grpcServer, apiServer.ServerHandler)
			},
			func(grpcServer *grpc.Server) {
				pb.RegisterServerAdminServiceServer(grpcServer, apiServer.ServerAdminHandler)
			},
			func(grpcServer *grpc.Server) {
				pb.RegisterWordServiceServer(grpcServer, apiServer.WordHandler)
			},
			func(grpcServer *grpc.Server) {
				pb.RegisterGameServiceServer(grpcServer, apiServer.GameHandler)
			},
		},
	)
}

func runGatewayServer(ctx context.Context, wg *errgroup.Group, apiServer *server.APIServer, deps *deps.APIServerDeps) {
	wsHandler := websocketserver.WsHandler{
		APIServerDeps: deps,
	}

	commonServer.RunGatewayServer(
		ctx,
		wg,
		deps.Config.HTTPServerAddress,
		deps.Config.IsDevelopmentEnvironment(),
		deps.Config.AllowedOrigins,
		[]commonServer.GatewayRouteRegistrar{
			func(ctx context.Context, mux *runtime.ServeMux) error {
				return pb.RegisterServerServiceHandlerServer(ctx, mux, apiServer.ServerHandler)
			},
			func(ctx context.Context, mux *runtime.ServeMux) error {
				return pb.RegisterServerAdminServiceHandlerServer(ctx, mux, apiServer.ServerAdminHandler)
			},
			func(ctx context.Context, mux *runtime.ServeMux) error {
				return pb.RegisterWordServiceHandlerServer(ctx, mux, apiServer.WordHandler)
			},
			func(ctx context.Context, mux *runtime.ServeMux) error {
				return pb.RegisterGameServiceHandlerServer(ctx, mux, apiServer.GameHandler)
			},
		},
		[]commonServer.HttpRouteRegistrar{
			func(mux *http.ServeMux) {
				mux.HandleFunc(websocketserver.WsServerJoinEndpoint, func(w http.ResponseWriter, r *http.Request) {
					if _, err := wsHandler.ServeWs(ctx, w, r); err != nil {
						log.Error().Err(err).Msg("could not serve server join ws")
					}
				})
			},
		},
		func(handler http.Handler) http.Handler {
			config := &commonMiddleware.AuthenticateServiceConfig{
				Cache: deps.Cache,
			}
			return commonMiddleware.AuthenticateServiceHTTP(handler, config)
		},
	)
}

func startGameServices(
	ctx context.Context,
	waitGroup *errgroup.Group,
	config util.Config,
	store db.Store,
	cache commonCache.Cache,
	bus eventbus.EventBus,
	taskDistributor worker.TaskDistributor,
) (gameflow.Client, *gameserver.Manager, *gamecache.GameCache, wordstore.Store) {
	gameCache := gamecache.New(config, cache)

	wordStore, err := wordstore.NewDefaultStore()
	if err != nil {
		log.Fatal().Err(err).Msg("could not initialize word store")
	}

	gfClient, err := gameflow.NewClient(config)
	if err != nil {
		log.Fatal().Err(err).Msg("could not create gameFlow client")
	}

	wk := gameflow.Worker{
		Config:          config,
		Store:           store,
		Bus:             bus,
		WordStore:       wordStore,
		TaskDistributor: taskDistributor,
	}
	waitGroup.Go(func() error {
		return wk.Run(ctx)
	})

	gsManager := gameserver.NewManager()

	waitGroup.Go(func() error {
		<-ctx.Done()

		gsManager.Shutdown()
		log.Info().Msg("game server manager stopped")

		gfClient.Close()
		log.Info().Msg("gameFlow client closed")

		return nil
	})

	return gfClient, gsManager, gameCache, wordStore
}

func runTaskProcessor(
	redisOpt asynq.RedisConnOpt,
	store db.Store,
	bus eventbus.EventBus,
	objectStore commonStorage.ObjectStore,
) worker.TaskProcessor {
	taskProcessor := worker.NewRedisTaskProcessor(redisOpt, bus, store, objectStore)

	err := taskProcessor.Start()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to start task processor")
	}

	log.Info().Msg("task processor started")
	return taskProcessor
}

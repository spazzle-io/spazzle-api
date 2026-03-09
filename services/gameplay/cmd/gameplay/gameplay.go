package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"

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

	redisOpt, err := asynq.ParseRedisURI(config.RedisConnURL)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to parse redis URI for asynq")
	}
	taskDistributor := worker.NewRedisTaskDistributor(redisOpt)

	bus, err := eventbus.New(ctx, config)
	if err != nil {
		log.Fatal().Err(err).Msg("could not create event bus")
	}

	wordStore, err := wordstore.NewDefaultStore()
	if err != nil {
		log.Fatal().Err(err).Msg("could not initialize word store")
	}

	authService, err := services.NewAuthServiceGrpcClient(config.AuthServiceGRPCServerAddr)
	if err != nil {
		log.Fatal().Err(err).Msg("could not create auth service client")
	}

	waitGroup, ctx := errgroup.WithContext(ctx)

	serverCfg := &server.APIServerConfig{
		Config:          config,
		Store:           store,
		Cache:           redisCache,
		Bus:             bus,
		WordStore:       wordStore,
		ObjectStore:     objectStore,
		TaskDistributor: taskDistributor,
		AuthService:     authService,
	}

	serverCfg.GfClient, serverCfg.GsManager = startGameServices(ctx, waitGroup, serverCfg)

	go runTaskProcessor(redisOpt, serverCfg)

	runGRPCServer(ctx, waitGroup, serverCfg)
	runGatewayServer(ctx, waitGroup, serverCfg)

	if err = waitGroup.Wait(); err != nil {
		log.Fatal().Err(err).Msg("could not wait for server shutdown")
	}

	stopInterruptCtx()

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

func runGRPCServer(ctx context.Context, wg *errgroup.Group, serverCfg *server.APIServerConfig) {
	s, err := server.NewAPIServer(serverCfg)
	if err != nil {
		log.Fatal().Err(err).Msg("could not create server")
	}

	commonServer.RunGRPCServer(
		ctx,
		wg,
		serverCfg.Config.GRPCServerAddress,
		[]commonServer.GrpcMiddlewareProvider{
			func() grpc.UnaryServerInterceptor {
				config := &commonMiddleware.AuthenticateServiceConfig{
					Cache: serverCfg.Cache,
				}
				return config.AuthenticateServiceGrpc
			},
		},
		[]commonServer.GrpcServiceRegistrar{
			func(grpcServer *grpc.Server) {
				pb.RegisterServerServiceServer(grpcServer, &s.ServerHandler)
			},
			func(grpcServer *grpc.Server) {
				pb.RegisterServerAdminServiceServer(grpcServer, &s.ServerAdminHandler)
			},
			func(grpcServer *grpc.Server) {
				pb.RegisterWordServiceServer(grpcServer, &s.WordHandler)
			},
			func(grpcServer *grpc.Server) {
				pb.RegisterGameServiceServer(grpcServer, &s.GameHandler)
			},
		},
	)
}

func runGatewayServer(ctx context.Context, wg *errgroup.Group, serverCfg *server.APIServerConfig) {
	s, err := server.NewAPIServer(serverCfg)
	if err != nil {
		log.Fatal().Err(err).Msg("could not create server")
	}

	wsHandler := websocketserver.WsHandler{
		Config:    serverCfg.Config,
		Store:     serverCfg.Store,
		Cache:     serverCfg.Cache,
		GfClient:  serverCfg.GfClient,
		Bus:       serverCfg.Bus,
		GsManager: serverCfg.GsManager,
		WordStore: serverCfg.WordStore,
	}

	commonServer.RunGatewayServer(
		ctx,
		wg,
		serverCfg.Config.HTTPServerAddress,
		serverCfg.Config.IsDevelopmentEnvironment(),
		serverCfg.Config.AllowedOrigins,
		[]commonServer.GatewayRouteRegistrar{
			func(ctx context.Context, mux *runtime.ServeMux) error {
				return pb.RegisterServerServiceHandlerServer(ctx, mux, &s.ServerHandler)
			},
			func(ctx context.Context, mux *runtime.ServeMux) error {
				return pb.RegisterServerAdminServiceHandlerServer(ctx, mux, &s.ServerAdminHandler)
			},
			func(ctx context.Context, mux *runtime.ServeMux) error {
				return pb.RegisterWordServiceHandlerServer(ctx, mux, &s.WordHandler)
			},
			func(ctx context.Context, mux *runtime.ServeMux) error {
				return pb.RegisterGameServiceHandlerServer(ctx, mux, &s.GameHandler)
			},
		},
		[]commonServer.HttpRouteRegistrar{
			func(mux *http.ServeMux) {
				mux.HandleFunc(websocketserver.WsServerJoinEndpoint, func(w http.ResponseWriter, r *http.Request) {
					_, err = wsHandler.ServeWs(ctx, w, r)
					if err != nil {
						log.Error().Err(err).Msg("could not serve server join ws")
					}
				})
			},
		},
		func(handler http.Handler) http.Handler {
			config := &commonMiddleware.AuthenticateServiceConfig{
				Cache: serverCfg.Cache,
			}
			return commonMiddleware.AuthenticateServiceHTTP(handler, config)
		},
	)
}

func startGameServices(
	ctx context.Context,
	waitGroup *errgroup.Group,
	cfg *server.APIServerConfig,
) (gameflow.Client, *gameserver.Manager) {
	gfClient, err := gameflow.NewClient(cfg.Config)
	if err != nil {
		log.Fatal().Err(err).Msg("could not create gameFlow client")
		return nil, nil
	}

	wk := gameflow.Worker{
		Config:          cfg.Config,
		Store:           cfg.Store,
		Bus:             cfg.Bus,
		WordStore:       cfg.WordStore,
		TaskDistributor: cfg.TaskDistributor,
	}
	wk.Start()

	gsManager := gameserver.NewManager()

	waitGroup.Go(func() error {
		<-ctx.Done()

		gsManager.Shutdown()
		log.Info().Msg("game server manager stopped")

		gfClient.Close()
		log.Info().Msg("gameFlow client closed")

		wk.Stop()
		log.Info().Msg("gameFlow worker stopped")

		return nil
	})

	return gfClient, gsManager
}

func runTaskProcessor(redisOpt asynq.RedisConnOpt, serverCfg *server.APIServerConfig) {
	taskProcessor := worker.NewRedisTaskProcessor(redisOpt, serverCfg.Bus, serverCfg.ObjectStore)

	err := taskProcessor.Start()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to start task processor")
	}

	log.Info().Msg("task processor started")
}

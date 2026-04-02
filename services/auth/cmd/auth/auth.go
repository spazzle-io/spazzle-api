package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	pb "github.com/spazzle-io/spazzle-api/services/proto/auth/auth/v1"

	commonCache "github.com/spazzle-io/spazzle-api/libs/common/cache"
	commonMiddleware "github.com/spazzle-io/spazzle-api/libs/common/middleware"

	"golang.org/x/sync/errgroup"

	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	_ "github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
	commonConfig "github.com/spazzle-io/spazzle-api/libs/common/config"
	_ "github.com/spazzle-io/spazzle-api/libs/common/docs/statik"
	commonServer "github.com/spazzle-io/spazzle-api/libs/common/server"
	"github.com/spazzle-io/spazzle-api/services/auth/internal/api/server"
	db "github.com/spazzle-io/spazzle-api/services/auth/internal/db/sqlc"
	"github.com/spazzle-io/spazzle-api/services/auth/internal/util"
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

	redisCache, err := commonCache.NewRedisCache(config.RedisConnURL)
	if err != nil {
		log.Fatal().Err(err).Msg("could not create redis cache")
	}

	apiServer, err := server.New(config, store, redisCache)
	if err != nil {
		log.Fatal().Err(err).Msg("could not create API server")
	}

	runGRPCServer(ctx, waitGroup, config, redisCache, apiServer)
	runGatewayServer(ctx, waitGroup, config, redisCache, apiServer)

	err = waitGroup.Wait()
	if err != nil {
		log.Fatal().Err(err).Msg("service terminating due to component failure")
	}

	stopInterruptCtx()

	err = redisCache.Close()
	if err != nil {
		log.Fatal().Err(err).Msg("could not close redis cache")
	}
}

func runGRPCServer(
	ctx context.Context,
	waitGroup *errgroup.Group,
	config util.Config,
	cache commonCache.Cache,
	apiServer *server.Server,
) {
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
		[]commonServer.GrpcServiceRegistrar{
			func(grpcServer *grpc.Server) {
				pb.RegisterAuthServiceServer(grpcServer, apiServer)
			},
		},
	)
}

func runGatewayServer(
	ctx context.Context,
	waitGroup *errgroup.Group,
	config util.Config,
	cache commonCache.Cache,
	apiServer *server.Server,
) {
	commonServer.RunGatewayServer(
		ctx,
		waitGroup,
		config.HTTPServerAddress,
		config.IsDevelopmentEnvironment(),
		config.AllowedOrigins,
		[]commonServer.GatewayRouteRegistrar{
			func(ctx context.Context, mux *runtime.ServeMux) error {
				return pb.RegisterAuthServiceHandlerServer(ctx, mux, apiServer)
			},
		},
		[]commonServer.HttpRouteRegistrar{},
		func(handler http.Handler) http.Handler {
			config := &commonMiddleware.AuthenticateServiceConfig{
				Cache: cache,
			}
			return commonMiddleware.AuthenticateServiceHTTP(handler, config)
		},
	)
}

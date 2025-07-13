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

	redisCache, err := commonCache.NewRedisCache(config.RedisConnURL)
	if err != nil {
		log.Fatal().Err(err).Msg("could not create redis cache")
	}

	waitGroup, ctx := errgroup.WithContext(ctx)

	runGRPCServer(ctx, waitGroup, config, store, redisCache)
	runGatewayServer(ctx, waitGroup, config, store, redisCache)

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

func runGRPCServer(
	ctx context.Context,
	waitGroup *errgroup.Group,
	config util.Config,
	store db.Store,
	cache commonCache.Cache,
) {
	s, err := server.New(config, store, cache)
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
		[]commonServer.GrpcServiceRegistrar{
			func(grpcServer *grpc.Server) {
				pb.RegisterAuthServiceServer(grpcServer, s)
			},
		},
	)
}

func runGatewayServer(
	ctx context.Context,
	waitGroup *errgroup.Group,
	config util.Config,
	store db.Store,
	cache commonCache.Cache,
) {
	s, err := server.New(config, store, cache)
	if err != nil {
		log.Fatal().Err(err).Msg("could not create server")
	}

	commonServer.RunGatewayServer(
		ctx,
		waitGroup,
		config.HTTPServerAddress,
		config.IsDevelopmentEnvironment(),
		config.AllowedOrigins,
		[]commonServer.GatewayRouteRegistrar{
			func(ctx context.Context, mux *runtime.ServeMux) error {
				return pb.RegisterAuthServiceHandlerServer(ctx, mux, s)
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

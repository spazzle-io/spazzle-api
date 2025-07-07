package main

import (
	"context"
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
	pb "github.com/spazzle-io/spazzle-api/services/proto/auth"
	"google.golang.org/grpc"
	"net/http"
)

func main() {
	config, err := commonConfig.LoadConfig[util.Config](".", ".development")
	if err != nil {
		log.Fatal().Err(err).Msg("could not load config")
	}

	commonConfig.SetupLogger(config.ServiceName, config.IsDevelopmentEnvironment())

	commonConfig.RunDBMigration(config.DBMigrationURL, config.DBSource)

	connPool, err := pgxpool.New(context.Background(), config.DBSource)
	if err != nil {
		log.Fatal().Err(err).Msg("could not connect to database")
	}
	store := db.NewStore(connPool)

	go runGRPCServer(config, store)
	runGatewayServer(config, store)
}

func runGRPCServer(config util.Config, store db.Store) {
	s, err := server.New(config, store)
	if err != nil {
		log.Fatal().Err(err).Msg("could not create server")
	}

	commonServer.RunGRPCServer(
		config.GRPCServerAddress,
		[]commonServer.GrpcMiddlewareProvider{},
		[]commonServer.GrpcServiceRegistrar{
			func(grpcServer *grpc.Server) {
				pb.RegisterAuthServer(grpcServer, s)
			},
		},
	)
}

func runGatewayServer(config util.Config, store db.Store) {
	s, err := server.New(config, store)
	if err != nil {
		log.Fatal().Err(err).Msg("could not create server")
	}

	commonServer.RunGatewayServer(
		config.HTTPServerAddress,
		config.IsDevelopmentEnvironment(),
		[]commonServer.GatewayRouteRegistrar{
			func(ctx context.Context, mux *runtime.ServeMux) error {
				return pb.RegisterAuthHandlerServer(ctx, mux, s)
			},
		},
		[]commonServer.HttpRouteRegistrar{},
		func(handler http.Handler) http.Handler {
			return handler
		},
	)
}

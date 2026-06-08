package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/spazzle-io/spazzle-api/services/users/internal/infra"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	pb "github.com/spazzle-io/spazzle-api/services/proto/users/users/v1"

	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/jackc/pgx/v5"
	"github.com/rs/zerolog/log"
	commonConfig "github.com/spazzle-io/spazzle-api/libs/common/config"
	_ "github.com/spazzle-io/spazzle-api/libs/common/docs/statik"
	commonMiddleware "github.com/spazzle-io/spazzle-api/libs/common/middleware"
	commonServer "github.com/spazzle-io/spazzle-api/libs/common/server"
	"github.com/spazzle-io/spazzle-api/services/users/internal/api/server"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
)

var interruptSignals = []os.Signal{
	os.Interrupt,
	syscall.SIGTERM,
}

func main() {
	if err := run(); err != nil {
		log.Fatal().Err(err).Msg("service terminated unexpectedly")
	}
}

func run() error {
	signalCtx, stopInterruptCtx := signal.NotifyContext(context.Background(), interruptSignals...)
	defer stopInterruptCtx()

	waitGroup, errGroupCtx := errgroup.WithContext(signalCtx)

	res, cleanupFunc, err := infra.NewResources(errGroupCtx)
	if err != nil {
		return fmt.Errorf("failed to initialize core service resources: %w", err)
	}
	defer cleanupFunc()

	apiServer, err := server.New(res)
	if err != nil {
		return fmt.Errorf("could not create API server: %w", err)
	}

	err = runGRPCServer(errGroupCtx, waitGroup, apiServer, res)
	if err != nil {
		return fmt.Errorf("could not start gRPC server: %w", err)
	}

	err = runGatewayServer(errGroupCtx, waitGroup, apiServer, res)
	if err != nil {
		return fmt.Errorf("could not start HTTP gateway server: %w", err)
	}

	if err = waitGroup.Wait(); err != nil {
		return fmt.Errorf("service terminating due to component failure: %w", err)
	}

	log.Info().Msg("service successfully stopped")
	return nil
}

func runGRPCServer(ctx context.Context, wg *errgroup.Group, apiServer *server.Server, res *infra.Resources) error {
	err := commonServer.RunGRPCServer(
		ctx,
		wg,
		res.Config.GRPCServerAddress,
		[]commonServer.GrpcMiddlewareProvider{
			func() grpc.UnaryServerInterceptor {
				config := &commonMiddleware.AuthenticateServiceConfig{
					Cache:  res.Cache,
					Config: &res.Config.AppConfig,
				}
				return config.AuthenticateServiceGrpc
			},
		},
		[]commonServer.GrpcServiceRegistrar{
			func(grpcServer *grpc.Server) {
				pb.RegisterUserServiceServer(grpcServer, apiServer)
			},
		},
	)
	if err != nil {
		return err
	}

	return nil
}

func runGatewayServer(ctx context.Context, wg *errgroup.Group, apiServer *server.Server, res *infra.Resources) error {
	err := commonServer.RunGatewayServer(
		ctx,
		wg,
		res.Config.HTTPServerAddress,
		res.Config.Is(commonConfig.Development),
		res.Config.AllowedOrigins,
		[]commonServer.GatewayRouteRegistrar{
			func(ctx context.Context, mux *runtime.ServeMux) error {
				return pb.RegisterUserServiceHandlerServer(ctx, mux, apiServer)
			},
		},
		[]commonServer.HttpRouteRegistrar{},
		func(handler http.Handler) http.Handler {
			config := &commonMiddleware.AuthenticateServiceConfig{
				Cache:  res.Cache,
				Config: &res.Config.AppConfig,
			}
			return commonMiddleware.AuthenticateServiceHTTP(handler, config)
		},
	)
	if err != nil {
		return err
	}

	return nil
}

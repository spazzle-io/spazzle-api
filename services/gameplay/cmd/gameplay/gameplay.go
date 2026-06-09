package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/infra"

	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/api/server/websocketserver"

	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	_ "github.com/jackc/pgx/v5"
	"github.com/rs/zerolog/log"
	commonConfig "github.com/spazzle-io/spazzle-api/libs/common/config"
	_ "github.com/spazzle-io/spazzle-api/libs/common/docs/statik"
	commonMiddleware "github.com/spazzle-io/spazzle-api/libs/common/middleware"
	commonServer "github.com/spazzle-io/spazzle-api/libs/common/server"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/api/server"
	pb "github.com/spazzle-io/spazzle-api/services/proto/gameplay/gameplay/v1"
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

	waitGroup.Go(func() error {
		log.Info().Msg("starting async task processor worker...")
		return res.TaskProcessor.StartWorker()
	})

	waitGroup.Go(func() error {
		log.Info().Msg("starting async task processor scheduler...")
		return res.TaskProcessor.StartScheduler()
	})

	waitGroup.Go(func() error {
		log.Info().Msg("starting gameflow worker...")
		return res.GfWorker.Run(errGroupCtx)
	})

	apiServer, err := server.NewAPIServer(res)
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

func runGRPCServer(ctx context.Context, wg *errgroup.Group, apiServer *server.APIServer, res *infra.Resources) error {
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
	if err != nil {
		return err
	}

	return nil
}

func runGatewayServer(ctx context.Context, wg *errgroup.Group, apiServer *server.APIServer, res *infra.Resources) error {
	err := commonServer.RunGatewayServer(
		ctx,
		wg,
		res.Config.HTTPServerAddress,
		res.Config.Is(commonConfig.Development),
		res.Config.AllowedOrigins,
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
					if _, err := websocketserver.ServeWs(ctx, res, w, r); err != nil {
						log.Error().Err(err).Msg("could not serve server join ws")
					}
				})
			},
		},
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

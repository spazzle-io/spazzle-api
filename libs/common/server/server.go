package server

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"

	"github.com/spazzle-io/spazzle-api/libs/common/middleware"

	"github.com/rs/cors"

	"golang.org/x/sync/errgroup"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/rakyll/statik/fs"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/encoding/protojson"
)

/*
GrpcMiddlewareProvider defines a function that returns a gRPC UnaryServerInterceptor.

Each service can provide its own middleware (e.g., authentication, logging, rate-limiting)
by returning it through this function.

Example:

	func provideLoggerMiddleware() server.GrpcMiddlewareProvider {
	    return func() grpc.UnaryServerInterceptor {
	        return middleware.GrpcLogger
	    }
	}
*/
type GrpcMiddlewareProvider func() grpc.UnaryServerInterceptor

/*
GrpcServiceRegistrar defines a function that registers a gRPC service implementation
onto a gRPC server instance.

Services use this to register their protobuf service implementations.

Example:

	func registerAuthService(s *AuthServer) server.GrpcServiceRegistrar {
	    return func(grpcServer *grpc.Server) {
	        pb.RegisterAuthServer(grpcServer, s)
	    }
	}
*/
type GrpcServiceRegistrar func(grpcServer *grpc.Server)

/*
GatewayRouteRegistrar defines a function that registers a gRPC-Gateway route
onto a runtime.ServeMux using the given context.

Services can use this to register their HTTP gateway handlers.

Example:

	func registerAuthGatewayHandler(s *AuthServer) server.GatewayRouteRegistrar {
	    return func(ctx context.Context, mux *runtime.ServeMux) error {
	        return pb.RegisterAuthHandlerServer(ctx, mux, s)
	    }
	}
*/
type GatewayRouteRegistrar func(ctx context.Context, mux *runtime.ServeMux) error

/*
HttpRouteRegistrar defines a function that registers additional raw HTTP routes (like /ws)
onto a standard net/http ServeMux.

Example:

	func registerWebSocketRoute() server.HttpRouteRegistrar {
	    return func(mux *http.ServeMux) {
	        mux.HandleFunc("/ws", handleWebSocket)
	    }
	}
*/
type HttpRouteRegistrar func(mux *http.ServeMux)

/*
HttpMiddlewareBuilder defines a function that wraps an HTTP handler
with additional middleware, such as logging, metrics, or auth.

This allows services to customize the HTTP middleware stack.

Example:

	func myMiddlewareBuilder(next http.Handler) http.Handler {
	    return middleware.HTTPLogger(middleware.HTTPRateLimiter(next))
	}
*/
type HttpMiddlewareBuilder func(http.Handler) http.Handler

/*
RunGRPCServer starts a gRPC server on the given address. It accepts a slice of
GrpcMiddlewareProvider to chain unary interceptors, and a slice of GrpcServiceRegistrar
to register service implementations.

Panics on failure to listen or serve.

Example usage:

	server.RunGRPCServer(
	    ":9090",
		[]GrpcMiddlewareProvider{
			func() grpc.UnaryServerInterceptor { return middleware.GrpcExtractMetadata },
			func() grpc.UnaryServerInterceptor { return middleware.GrpcLogger },
		},
		[]GrpcServiceRegistrar{
			func(grpcServer *grpc.Server) {
				pb.RegisterAuthServer(grpcServer, srv)
			},
		},
	)
*/
func RunGRPCServer(
	ctx context.Context,
	waitGroup *errgroup.Group,
	address string,
	middlewareProviders []GrpcMiddlewareProvider,
	serviceRegistrars []GrpcServiceRegistrar,
) {
	interceptors := []grpc.UnaryServerInterceptor{
		middleware.GrpcExtractMetadata,
		middleware.GrpcRateLimiter,
		middleware.GrpcLogger,
	}

	for _, provider := range middlewareProviders {
		interceptors = append(interceptors, provider())
	}

	grpcServer := grpc.NewServer(grpc.ChainUnaryInterceptor(interceptors...))

	for _, registrar := range serviceRegistrars {
		registrar(grpcServer)
	}
	reflection.Register(grpcServer)

	listener, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create gRPC listener")
	}

	waitGroup.Go(func() error {
		log.Info().Msgf("started gRPC server at %s", listener.Addr().String())

		if err := grpcServer.Serve(listener); err != nil {
			if errors.Is(err, grpc.ErrServerStopped) {
				return nil
			}
			log.Error().Err(err).Msg("failed to serve gRPC")
			return err
		}

		return nil
	})

	waitGroup.Go(func() error {
		<-ctx.Done()
		log.Info().Msg("shutting down gRPC server")

		grpcServer.GracefulStop()
		log.Info().Msg("gRPC server stopped")

		return nil
	})
}

/*
RunGatewayServer starts an HTTP server with gRPC-Gateway routing, optional HTTP routes,
and a custom HTTP middleware stack. It also mounts Swagger docs in development mode.

Panics on failure to listen or serve.

Parameters:
  - address: HTTP server listen address
  - isDevelopmentEnvironment: enables Swagger serving if true
  - routeRegistrars: gRPC-Gateway HTTP route registration functions
  - httpRouteRegistrars: standard HTTP route registration functions (e.g. WebSocket handlers)
  - middlewareBuilder: function to wrap the final HTTP handler with middleware

Example usage:

	server.RunGatewayServer(
	    ":8080",
	    true,
	    []server.GatewayRouteRegistrar{
	        registerAuthGatewayHandler(authServer),
	    },
	    []server.HttpRouteRegistrar{
	        registerWebSocketRoute(),
	    },
	    myMiddlewareBuilder,
	)
*/
func RunGatewayServer(
	ctx context.Context,
	waitGroup *errgroup.Group,
	address string,
	isDevelopmentEnvironment bool,
	allowedOrigins []string,
	routeRegistrars []GatewayRouteRegistrar,
	httpRouteRegistrars []HttpRouteRegistrar,
	middlewareBuilder HttpMiddlewareBuilder,
) {
	opt := runtime.WithMarshalerOption(runtime.MIMEWildcard, &runtime.JSONPb{
		MarshalOptions: protojson.MarshalOptions{
			EmitDefaultValues: true,
			UseProtoNames:     true,
		},
		UnmarshalOptions: protojson.UnmarshalOptions{
			DiscardUnknown: true,
		},
	})

	grpcMux := runtime.NewServeMux(opt)

	for _, registrar := range routeRegistrars {
		if err := registrar(ctx, grpcMux); err != nil {
			log.Fatal().Err(err).Msg("failed to register gateway route")
		}
	}

	mux := http.NewServeMux()
	mux.Handle("/", grpcMux)

	for _, register := range httpRouteRegistrars {
		register(mux)
	}

	if isDevelopmentEnvironment {
		mux = serveSwagger(mux)
	}

	handler := middleware.HTTPExtractMetadata(
		middleware.HTTPRateLimiter(
			middleware.HTTPLogger(
				middlewareBuilder(mux),
			),
		),
	)

	c := cors.New(cors.Options{
		AllowedOrigins: allowedOrigins,
		AllowedMethods: []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodPatch,
			http.MethodDelete,
		},
		AllowedHeaders:   []string{"*"},
		AllowCredentials: true,
	})
	handler = c.Handler(handler)

	srv := &http.Server{
		Handler:      handler,
		Addr:         address,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	waitGroup.Go(func() error {
		log.Info().Msgf("started HTTP gateway server at %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil {
			if errors.Is(err, http.ErrServerClosed) {
				return nil
			}
			log.Error().Err(err).Msg("failed to serve HTTP")
			return err
		}

		return nil
	})

	waitGroup.Go(func() error {
		<-ctx.Done()
		log.Info().Msg("shutting down HTTP server")

		if err := srv.Shutdown(context.Background()); err != nil {
			log.Error().Err(err).Msg("failed to shutdown HTTP server")
		}

		log.Info().Msg("HTTP server stopped")
		return nil
	})
}

func serveSwagger(mux *http.ServeMux) *http.ServeMux {
	statikFS, err := fs.New()
	if err != nil {
		log.Fatal().Err(err).Msg("cannot create statik fs")
	}

	swaggerHandler := http.StripPrefix("/swagger/", http.FileServer(statikFS))
	mux.Handle("/swagger/", swaggerHandler)

	return mux
}

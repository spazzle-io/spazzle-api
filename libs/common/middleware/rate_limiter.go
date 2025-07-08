package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
	"github.com/ulule/limiter/v3"
	"github.com/ulule/limiter/v3/drivers/middleware/stdlib"
	redisStore "github.com/ulule/limiter/v3/drivers/store/redis"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type Rate struct {
	Limit      int64
	Period     time.Duration
	Identifier string
	Aliases    []string
}

const DefaultRateLimitIdentifier = "default_rate_limit"

var activeRateLimits = map[string]Rate{
	// default global rate limit:
	// applies to all endpoints collectively when no specific rate limit is defined for an individual endpoint.
	DefaultRateLimitIdentifier: {Limit: 1000, Period: time.Hour, Identifier: DefaultRateLimitIdentifier},
}

var limiters = make(map[string]*limiter.Limiter)

func CreateLimiterRedisStore(serviceName string, redisConnURL string) (limiter.Store, error) {
	opts, err := redis.ParseURL(redisConnURL)
	if err != nil {
		return nil, fmt.Errorf("could not parse redis connection url: %w", err)
	}

	rc := redis.NewClient(opts)
	store, err := redisStore.NewStoreWithOptions(rc, limiter.StoreOptions{
		Prefix: fmt.Sprintf("%s-api-rate-limiter", serviceName),
	})
	if err != nil {
		return nil, fmt.Errorf("could not create a new redis rate limiter store: %w", err)
	}

	return store, nil
}

func InitializeLimiters(store limiter.Store, rateLimits map[string]Rate) error {
	limiters = make(map[string]*limiter.Limiter)

	for k, v := range rateLimits {
		activeRateLimits[k] = v
		for _, alias := range v.Aliases {
			activeRateLimits[alias] = Rate{
				Limit:      v.Limit,
				Period:     v.Period,
				Identifier: v.Identifier,
				Aliases:    append(v.Aliases, k),
			}
		}
	}

	for _, rateLimit := range activeRateLimits {
		_, exists := limiters[rateLimit.Identifier]
		if exists {
			continue
		}

		r := limiter.Rate{
			Limit:  rateLimit.Limit,
			Period: rateLimit.Period,
		}

		l := limiter.New(store, r, limiter.WithTrustForwardHeader(true))

		limiters[rateLimit.Identifier] = l
	}

	return nil
}

func getEndpointRateLimit(endpoint string) Rate {
	rateLimit, exists := activeRateLimits[endpoint]
	if !exists {
		rateLimit = activeRateLimits[DefaultRateLimitIdentifier]
	}

	return rateLimit
}

func getLimiter(r Rate) (*limiter.Limiter, error) {
	l := limiters[r.Identifier]
	if l == nil {
		return nil, fmt.Errorf("could not get rate limiter for identifier: %s", r.Identifier)
	}

	return l, nil
}

var getLimiterContext = func(ctx context.Context, l *limiter.Limiter, key string) (limiter.Context, error) {
	return l.Get(ctx, key)
}

func GrpcRateLimiter(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (resp interface{}, err error) {
	// TODO: Skip rate limits for authenticated services

	endpoint := info.FullMethod
	logger := log.With().Str("endpoint", endpoint).Logger()

	rateLimit := getEndpointRateLimit(endpoint)
	l, err := getLimiter(rateLimit)
	if err != nil {
		logger.Error().Err(err).Msg("could not get rate limiter")
		return nil, status.Error(codes.Internal, InternalServerError)
	}

	clientIP, ok := ctx.Value(ClientIP).(string)
	if !ok {
		logger.Error().Msg("could not extract client IP for rate limiting")
		return nil, status.Error(codes.InvalidArgument, MissingXForwardedForHeaderError)
	}

	key := fmt.Sprintf("%s:%s", rateLimit.Identifier, clientIP)
	logger = logger.With().Str("client_ip", clientIP).Logger()

	c, err := getLimiterContext(ctx, l, key)
	if err != nil {
		logger.Error().Err(err).Msg("could not get rate limiter context")
		return nil, status.Error(codes.Internal, InternalServerError)
	}

	headers := metadata.Pairs(
		xRateLimitLimitHeader, strconv.FormatInt(c.Limit, 10),
		xRateLimitRemainingHeader, strconv.FormatInt(c.Remaining, 10),
		xRateLimitResetHeader, strconv.FormatInt(c.Reset, 10),
	)
	err = grpc.SendHeader(ctx, headers)
	if err != nil {
		logger.Error().Err(err).Msg("could not send rate limit headers")
		return nil, status.Error(codes.Internal, InternalServerError)
	}

	if c.Reached {
		logger.Error().Msg("rate limit exceeded for client IP")
		return nil, status.Error(codes.ResourceExhausted, RateLimitExceededError)
	}

	return handler(ctx, req)
}

type ErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func httpError(res http.ResponseWriter, grpcError error, httpStatusCode int) {
	var errorResponse ErrorResponse

	st, ok := status.FromError(grpcError)
	if !ok {
		errorResponse = ErrorResponse{
			Code:    httpStatusCode,
			Message: InternalServerError,
		}
	} else {
		errorResponse = ErrorResponse{
			Code:    int(st.Code()),
			Message: st.Message(),
		}
	}

	res.Header().Set(ContentTypeHeader, applicationJSONValue)

	res.WriteHeader(httpStatusCode)
	err := json.NewEncoder(res).Encode(errorResponse)
	if err != nil {
		http.Error(res, errorResponse.Message, httpStatusCode)
		return
	}
}

func HTTPRateLimiter(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		// TODO: Skip rate limits for authenticated services

		endpoint := req.URL.Path
		logger := log.With().Str("endpoint", endpoint).Logger()

		rateLimit := getEndpointRateLimit(fmt.Sprintf("%s:%s", req.Method, endpoint))
		l, err := getLimiter(rateLimit)
		if err != nil {
			logger.Error().Err(err).Msg("could not get rate limiter")
			grpcError := status.Error(codes.Internal, InternalServerError)
			httpError(res, grpcError, http.StatusInternalServerError)
			return
		}

		middleware := stdlib.NewMiddleware(l)

		clientIP := l.GetIPKey(req)
		if clientIP == "" {
			logger.Error().Msg("could not extract client IP for rate limiting")
			grpcError := status.Error(codes.InvalidArgument, MissingXForwardedForHeaderError)
			httpError(res, grpcError, http.StatusBadRequest)
			return
		}

		middleware.KeyGetter = func(req *http.Request) string {
			return fmt.Sprintf("%s:%s", rateLimit.Identifier, clientIP)
		}

		logger = logger.With().Str("client_ip", clientIP).Logger()

		middleware.OnLimitReached = func(w http.ResponseWriter, r *http.Request) {
			logger.Error().Msg("rate limit exceeded for client IP")
			grpcError := status.Error(codes.ResourceExhausted, RateLimitExceededError)
			httpError(res, grpcError, http.StatusTooManyRequests)
		}

		handlerWithMiddleware := middleware.Handler(handler)
		handlerWithMiddleware.ServeHTTP(res, req)
	})
}

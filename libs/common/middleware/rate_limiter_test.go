package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/ulule/limiter/v3"
	"github.com/ulule/limiter/v3/drivers/store/memory"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func TestInitializeLimiters(t *testing.T) {
	testCases := []struct {
		name                string
		rateLimits          map[string]Rate
		expectedNumLimiters int
	}{
		{
			name:                "only default rate limit",
			rateLimits:          map[string]Rate{},
			expectedNumLimiters: 1,
		},
		{
			name: "with additional endpoint rate limits",
			rateLimits: map[string]Rate{
				"/pb.Auth/GetChallenge":        {Limit: 100, Period: time.Hour, Identifier: "gRPC Challenge"},
				"GET:/auth/accounts/challenge": {Limit: 100, Period: time.Hour, Identifier: "HTTP Challenge"},
			},
			expectedNumLimiters: 3,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			store := memory.NewStore()

			initialRateLimits := activeRateLimits
			defer func() {
				activeRateLimits = initialRateLimits
				limiters = make(map[string]*limiter.Limiter)
			}()

			err := InitializeLimiters(store, tc.rateLimits)
			require.NoError(t, err)
			require.Len(t, limiters, tc.expectedNumLimiters)
		})
	}
}

func TestGetEndpointRateLimit(t *testing.T) {
	testRateLimit := Rate{Limit: 1000, Period: time.Hour, Identifier: "Test"}

	testCases := []struct {
		name                        string
		endpoint                    string
		rateLimits                  map[string]Rate
		expectedRateLimitIdentifier string
	}{
		{
			name:                        "relies on default rate limit",
			endpoint:                    "/test",
			rateLimits:                  map[string]Rate{},
			expectedRateLimitIdentifier: DefaultRateLimitIdentifier,
		},
		{
			name:     "has a specific rate limit",
			endpoint: "GET:/test",
			rateLimits: map[string]Rate{
				"GET:/test": testRateLimit,
			},
			expectedRateLimitIdentifier: testRateLimit.Identifier,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			store := memory.NewStore()

			initialRateLimits := activeRateLimits
			defer func() {
				activeRateLimits = initialRateLimits
				limiters = make(map[string]*limiter.Limiter)
			}()

			err := InitializeLimiters(store, tc.rateLimits)
			require.NoError(t, err)

			rateLimit := getEndpointRateLimit(tc.endpoint)
			require.Equal(t, tc.expectedRateLimitIdentifier, rateLimit.Identifier)
		})
	}
}

func TestGetLimiter(t *testing.T) {
	store := memory.NewStore()

	testRateLimit := Rate{Limit: 1000, Period: time.Hour, Identifier: "Test"}
	uninitializedRateLimit := Rate{Limit: 1000, Period: time.Hour, Identifier: "Uninitialized"}

	initialRateLimits := activeRateLimits
	defer func() {
		activeRateLimits = initialRateLimits
		limiters = make(map[string]*limiter.Limiter)
	}()

	err := InitializeLimiters(store, map[string]Rate{
		"/test": testRateLimit,
	})
	require.NoError(t, err)

	testCases := []struct {
		name          string
		rateLimit     Rate
		expectLimiter bool
	}{
		{
			name:          "initialized rate limit",
			rateLimit:     testRateLimit,
			expectLimiter: true,
		},
		{
			name:          "uninitialized rate limit",
			rateLimit:     uninitializedRateLimit,
			expectLimiter: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			l, err := getLimiter(tc.rateLimit)
			if !tc.expectLimiter {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, l)
		})
	}
}

func TestOverrideRateLimit(t *testing.T) {
	store := memory.NewStore()

	initialRateLimits := activeRateLimits
	defer func() {
		activeRateLimits = initialRateLimits
		limiters = make(map[string]*limiter.Limiter)
	}()

	initial := map[string]Rate{"Test": {Limit: 1000, Period: time.Hour, Identifier: "Test"}}
	override := map[string]Rate{"Test": {Limit: 2000, Period: time.Minute, Identifier: "Test"}}

	err := InitializeLimiters(store, initial)
	require.NoError(t, err)

	rl := getEndpointRateLimit("Test")
	require.NotEmpty(t, rl)
	require.Equal(t, initial["Test"].Period, rl.Period)
	require.Equal(t, initial["Test"].Limit, rl.Limit)
	require.Equal(t, initial["Test"].Identifier, rl.Identifier)

	err = InitializeLimiters(store, override)
	require.NoError(t, err)

	rl = getEndpointRateLimit("Test")
	require.NotEmpty(t, rl)
	require.Equal(t, override["Test"].Period, rl.Period)
	require.Equal(t, override["Test"].Limit, rl.Limit)
	require.Equal(t, override["Test"].Identifier, rl.Identifier)
}

func TestRateLimitAliases(t *testing.T) {
	store := memory.NewStore()

	initialRateLimits := activeRateLimits
	defer func() {
		activeRateLimits = initialRateLimits
		limiters = make(map[string]*limiter.Limiter)
	}()

	err := InitializeLimiters(store, map[string]Rate{
		"test": {Limit: 10, Period: time.Hour, Identifier: "test", Aliases: []string{"test2"}},
	})
	require.NoError(t, err)

	rl := getEndpointRateLimit("test")
	require.NotEmpty(t, rl)
	require.Equal(t, rl.Limit, int64(10))
	require.Equal(t, rl.Period, time.Hour)
	require.Equal(t, rl.Identifier, "test")
	require.Equal(t, rl.Aliases, []string{"test2"})

	rl = getEndpointRateLimit("test2")
	require.NotEmpty(t, rl)
	require.Equal(t, rl.Limit, int64(10))
	require.Equal(t, rl.Period, time.Hour)
	require.Equal(t, rl.Identifier, "test")
	require.Equal(t, rl.Aliases, []string{"test2", "test"})
}

type mockUnaryHandler struct {
	resp interface{}
	err  error
}

func (m *mockUnaryHandler) mockHandle(_ context.Context, _ interface{}) (interface{}, error) {
	return m.resp, m.err
}

type mockServerTransportStream struct{}

func (m *mockServerTransportStream) Method() string {
	return "foo"
}

func (m *mockServerTransportStream) SetHeader(_ metadata.MD) error {
	return nil
}

func (m *mockServerTransportStream) SendHeader(_ metadata.MD) error {
	return nil
}

func (m *mockServerTransportStream) SetTrailer(_ metadata.MD) error {
	return nil
}

func TestGrpcRateLimiter(t *testing.T) {
	testCases := []struct {
		name               string
		rateLimits         map[string]Rate
		clientIP           string
		limiterContext     limiter.Context
		getLimiterCtxError error
		expectedError      string
	}{
		{
			name:           "valid request",
			rateLimits:     map[string]Rate{},
			clientIP:       "127.0.0.1",
			limiterContext: limiter.Context{Limit: 10, Remaining: 9, Reset: time.Now().Add(1 * time.Minute).Unix(), Reached: false},
			expectedError:  "",
		},
		{
			name:           "exceeded rate limit",
			rateLimits:     map[string]Rate{},
			clientIP:       "127.0.0.1",
			limiterContext: limiter.Context{Limit: 10, Remaining: 0, Reset: time.Now().Add(1 * time.Minute).Unix(), Reached: true},
			expectedError:  RateLimitExceededError,
		},
		{
			name:           "missing x-forwarded-for header",
			rateLimits:     map[string]Rate{},
			clientIP:       "",
			limiterContext: limiter.Context{Limit: 10, Remaining: 9, Reset: time.Now().Add(1 * time.Minute).Unix(), Reached: false},
			expectedError:  MissingXForwardedForHeaderError,
		},
		{
			name:               "could not get limiter context",
			rateLimits:         map[string]Rate{},
			clientIP:           "127.0.0.1",
			limiterContext:     limiter.Context{Limit: 10, Remaining: 9, Reset: time.Now().Add(1 * time.Minute).Unix(), Reached: false},
			getLimiterCtxError: errors.New("some error"),
			expectedError:      InternalServerError,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			store := memory.NewStore()

			initialRateLimits := activeRateLimits
			defer func() {
				activeRateLimits = initialRateLimits
				limiters = make(map[string]*limiter.Limiter)
			}()

			err := InitializeLimiters(store, tc.rateLimits)
			require.NoError(t, err)

			// add x-forwarded header to incoming context
			ctx := grpc.NewContextWithServerTransportStream(context.Background(), &mockServerTransportStream{})
			if tc.clientIP != "" {
				ctx = context.WithValue(ctx, ClientIP, tc.clientIP)
			}
			ctxWithHeader := metadata.NewIncomingContext(ctx, metadata.Pairs())

			initialGetLimiterContext := getLimiterContext
			getLimiterContext = func(ctx context.Context, l *limiter.Limiter, key string) (limiter.Context, error) {
				return tc.limiterContext, tc.getLimiterCtxError
			}
			defer func() {
				getLimiterContext = initialGetLimiterContext
			}()

			mockHandler := &mockUnaryHandler{
				resp: "success",
				err:  nil,
			}

			_, err = GrpcRateLimiter(ctxWithHeader, nil, &grpc.UnaryServerInfo{
				FullMethod: "/test",
			}, mockHandler.mockHandle)

			if tc.expectedError == "" {
				require.NoError(t, err)
				return
			}

			require.Error(t, err)
			require.ErrorContains(t, err, tc.expectedError)
		})
	}
}

func TestHTTPRateLimiter(t *testing.T) {
	testCases := []struct {
		name                 string
		rateLimits           map[string]Rate
		clientIP             string
		expectedResponseCode int
	}{
		{
			name:                 "valid request",
			rateLimits:           map[string]Rate{},
			clientIP:             "127.0.0.1",
			expectedResponseCode: http.StatusOK,
		},
		{
			name:                 "exceeded rate limit",
			rateLimits:           map[string]Rate{"GET:/test": {Limit: 0, Period: time.Hour, Identifier: "GET:/test"}},
			clientIP:             "127.0.0.1",
			expectedResponseCode: http.StatusTooManyRequests,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			store := memory.NewStore()

			initialRateLimits := activeRateLimits
			defer func() {
				activeRateLimits = initialRateLimits
				limiters = make(map[string]*limiter.Limiter)
			}()

			err := InitializeLimiters(store, tc.rateLimits)
			require.NoError(t, err)

			req, err := http.NewRequest("GET", "/test", nil)
			if err != nil {
				t.Fatal(err)
			}

			req.Header.Set(XForwardedForHeader, tc.clientIP)

			res := httptest.NewRecorder()

			HTTPRateLimiter(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				res.WriteHeader(http.StatusOK)
				_, err := res.Write([]byte("OK"))
				require.NoError(t, err)
			})).ServeHTTP(res, req)

			require.Equal(t, tc.expectedResponseCode, res.Code)
		})
	}
}

package middleware

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/spazzle-io/spazzle-api/libs/common/config"

	"github.com/spf13/viper"

	mockcache "github.com/spazzle-io/spazzle-api/libs/common/cache/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const (
	testServicePublicKeyPEM  = "MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEkcpsUaeko+BLe9sutR3FRCIQPBwlRU9UN2/69Q4RLb8upVzVcK+22dEJtvVzhu3bl1hgPk3HLIYPrtuLqKOQbw=="
	testServicePrivateKeyPEM = "MHcCAQEEINIZr7eRHNKIo+kqyLU5j8Y3mRmfn+5k2OY685DzM1MOoAoGCCqGSM49AwEHoUQDQgAEkcpsUaeko+BLe9sutR3FRCIQPBwlRU9UN2/69Q4RLb8upVzVcK+22dEJtvVzhu3bl1hgPk3HLIYPrtuLqKOQbw=="
)

func newTestAppConfig(t *testing.T, keys map[string]string) *config.AppConfig {
	t.Helper()

	v := viper.New()
	for k, val := range keys {
		v.Set(k, val)
	}

	return config.NewTestAppConfig(v)
}

func TestAuthenticateService(t *testing.T) {
	oneMinuteAgo := time.Now().UTC().Add(-1 * serviceAuthenticationPayloadDuration)
	oneMinuteAgoUTCMillis := oneMinuteAgo.UnixNano() / int64(time.Millisecond)
	currentTimeUTCMillis := time.Now().UTC().UnixNano() / int64(time.Millisecond)

	appCfg := newTestAppConfig(t, map[string]string{
		"SERVICE":                    "users",
		"SERVICE_USERS_PRIVATE_KEYS": testServicePrivateKeyPEM,
	})

	validPayload, err := GenerateServiceAuthenticationPayload(appCfg)
	require.NoError(t, err)
	require.NotEmpty(t, validPayload)

	testCases := []struct {
		name                  string
		inputContext          context.Context
		expectedResultContext context.Context
		buildStubs            func(cache *mockcache.MockCache)
	}{
		{
			name:                  "successfully authenticates service",
			inputContext:          context.WithValue(context.Background(), ServiceAuthentication, validPayload),
			expectedResultContext: context.WithValue(context.WithValue(context.Background(), ServiceAuthentication, validPayload), AuthenticatedService, "users"),
			buildStubs: func(cache *mockcache.MockCache) {
				cache.EXPECT().
					Get(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(ctx context.Context, key string, dest *string) error {
						*dest = ""
						return nil
					})

				cache.EXPECT().
					Set(gomock.Any(), gomock.Any(), gomock.Any(), serviceAuthenticationPayloadDuration).
					Times(1).
					Return(nil)
			},
		},
		{
			name:                  "input context lacks service_authentication value",
			inputContext:          context.Background(),
			expectedResultContext: context.Background(),
			buildStubs: func(cache *mockcache.MockCache) {
			},
		},
		{
			name:                  "invalid service authentication payload",
			inputContext:          context.WithValue(context.Background(), ServiceAuthentication, "invalid.payload"),
			expectedResultContext: context.WithValue(context.Background(), ServiceAuthentication, "invalid.payload"),
			buildStubs: func(cache *mockcache.MockCache) {
			},
		},
		{
			name:                  "invalid service authentication request timestamp",
			inputContext:          context.WithValue(context.Background(), ServiceAuthentication, "users.a.nPLZLG2JNI.dummybase64signature/+=="),
			expectedResultContext: context.WithValue(context.Background(), ServiceAuthentication, "users.a.nPLZLG2JNI.dummybase64signature/+=="),
			buildStubs: func(cache *mockcache.MockCache) {
			},
		},
		{
			name:                  "service name not provided",
			inputContext:          context.WithValue(context.Background(), ServiceAuthentication, ".1.nPLZLG2JNI.dummybase64signature/+=="),
			expectedResultContext: context.WithValue(context.Background(), ServiceAuthentication, ".1.nPLZLG2JNI.dummybase64signature/+=="),
			buildStubs: func(cache *mockcache.MockCache) {
			},
		},
		{
			name:                  "expired service authentication payload",
			inputContext:          context.WithValue(context.Background(), ServiceAuthentication, fmt.Sprintf("users.%d.nPLZLG2JNI.dummybase64signature/+==", oneMinuteAgoUTCMillis)),
			expectedResultContext: context.WithValue(context.Background(), ServiceAuthentication, fmt.Sprintf("users.%d.nPLZLG2JNI.dummybase64signature/+==", oneMinuteAgoUTCMillis)),
			buildStubs: func(cache *mockcache.MockCache) {
			},
		},
		{
			name:                  "invalid service authentication nonce",
			inputContext:          context.WithValue(context.Background(), ServiceAuthentication, fmt.Sprintf("users.%d.AbC456789.dummybase64signature/+==", currentTimeUTCMillis)),
			expectedResultContext: context.WithValue(context.Background(), ServiceAuthentication, fmt.Sprintf("users.%d.AbC456789.dummybase64signature/+==", currentTimeUTCMillis)),
			buildStubs: func(cache *mockcache.MockCache) {
			},
		},
		{
			name:                  "invalid service authentication signature",
			inputContext:          context.WithValue(context.Background(), ServiceAuthentication, fmt.Sprintf("users.%d.nPLZLG2JNI.invalidSignature/+==", currentTimeUTCMillis)),
			expectedResultContext: context.WithValue(context.Background(), ServiceAuthentication, fmt.Sprintf("users.%d.nPLZLG2JNI.invalidSignature/+==", currentTimeUTCMillis)),
			buildStubs: func(cache *mockcache.MockCache) {
			},
		},
		{
			name:                  "service authentication signature exists in cache",
			inputContext:          context.WithValue(context.Background(), ServiceAuthentication, validPayload),
			expectedResultContext: context.WithValue(context.Background(), ServiceAuthentication, validPayload),
			buildStubs: func(cache *mockcache.MockCache) {
				cache.EXPECT().
					Get(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(ctx context.Context, key string, dest *string) error {
						*dest = "some-value"
						return nil
					})
			},
		},
		{
			name:                  "cache Get() throws an error",
			inputContext:          context.WithValue(context.Background(), ServiceAuthentication, validPayload),
			expectedResultContext: context.WithValue(context.Background(), ServiceAuthentication, validPayload),
			buildStubs: func(cache *mockcache.MockCache) {
				cache.EXPECT().
					Get(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(ctx context.Context, key string, dest *string) error {
						*dest = ""
						return errors.New("some cache error")
					})
			},
		},
		{
			name:                  "cache Set() throws an error",
			inputContext:          context.WithValue(context.Background(), ServiceAuthentication, validPayload),
			expectedResultContext: context.WithValue(context.Background(), ServiceAuthentication, validPayload),
			buildStubs: func(cache *mockcache.MockCache) {
				cache.EXPECT().
					Get(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(ctx context.Context, key string, dest *string) error {
						*dest = ""
						return nil
					})

				cache.EXPECT().
					Set(gomock.Any(), gomock.Any(), gomock.Any(), serviceAuthenticationPayloadDuration).
					Times(1).
					Return(errors.New("some cache error"))
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			cache := mockcache.NewMockCache(ctrl)
			tc.buildStubs(cache)

			cfg := &AuthenticateServiceConfig{
				Cache: cache,
				Config: newTestAppConfig(t, map[string]string{
					"SERVICE_USERS_PUBLIC_KEYS": testServicePublicKeyPEM,
				}),
			}

			resultContext := authenticateService(tc.inputContext, cfg)
			require.Equal(t, tc.expectedResultContext, resultContext)
		})
	}
}

func TestGenerateServiceAuthenticationPayload_noPrivateKeysProvided(t *testing.T) {
	testCases := []struct {
		name            string
		config          *config.AppConfig
		expectedToError bool
	}{
		{
			name: "empty private keys env string",
			config: newTestAppConfig(t, map[string]string{
				"SERVICE":                    "users",
				"SERVICE_USERS_PRIVATE_KEYS": "",
			}),
			expectedToError: true,
		},
		{
			name: "private keys env string not provided",
			config: newTestAppConfig(t, map[string]string{
				"SERVICE": "users",
			}),
			expectedToError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			payload, err := GenerateServiceAuthenticationPayload(tc.config)
			require.Error(t, err)
			require.Empty(t, payload)
		})
	}
}

func TestGenerateServiceAuthenticationPayload_couldNotParsePEM(t *testing.T) {
	appCfg := newTestAppConfig(t, map[string]string{
		"SERVICE":                    "users",
		"SERVICE_USERS_PRIVATE_KEYS": "invalid_PEM",
	})

	payload, err := GenerateServiceAuthenticationPayload(appCfg)
	require.Error(t, err)
	require.Empty(t, payload)
}

func TestGenerateServiceAuthenticationPayload_success(t *testing.T) {
	appCfg := newTestAppConfig(t, map[string]string{
		"SERVICE":                    "users",
		"SERVICE_USERS_PRIVATE_KEYS": testServicePrivateKeyPEM,
	})

	payload, err := GenerateServiceAuthenticationPayload(appCfg)
	require.NoError(t, err)
	require.NotEmpty(t, payload)

	expectedPayloadPattern := `^users\.\d+\.[a-zA-Z0-9]{10}\.[a-zA-Z0-9+/=]+$`
	regex := regexp.MustCompile(expectedPayloadPattern)

	require.True(t, regex.MatchString(payload))
}

package middleware

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"testing"
	"time"

	mockcache "github.com/spazzle-io/spazzle-api/libs/common/cache/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const (
	testServicePublicKeyPEM  = "MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEkcpsUaeko+BLe9sutR3FRCIQPBwlRU9UN2/69Q4RLb8upVzVcK+22dEJtvVzhu3bl1hgPk3HLIYPrtuLqKOQbw=="
	testServicePrivateKeyPEM = "MHcCAQEEINIZr7eRHNKIo+kqyLU5j8Y3mRmfn+5k2OY685DzM1MOoAoGCCqGSM49AwEHoUQDQgAEkcpsUaeko+BLe9sutR3FRCIQPBwlRU9UN2/69Q4RLb8upVzVcK+22dEJtvVzhu3bl1hgPk3HLIYPrtuLqKOQbw=="
)

func TestAuthenticateService(t *testing.T) {
	oneMinuteAgo := time.Now().UTC().Add(-1 * serviceAuthenticationPayloadDuration)
	oneMinuteAgoUTCMillis := oneMinuteAgo.UnixNano() / int64(time.Millisecond)
	currentTimeUTCMillis := time.Now().UTC().UnixNano() / int64(time.Millisecond)

	initialGetViperStringSliceFunc := getViperStringSlice
	defer func() {
		getViperStringSlice = initialGetViperStringSliceFunc
	}()

	getViperStringSlice = func(_ string) []string {
		return []string{testServicePrivateKeyPEM}
	}

	validPayload, err := GenerateServiceAuthenticationPayload("UsERs")
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
					Get(gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil, nil)

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
					Get(gomock.Any(), gomock.Any()).
					Times(1).
					Return("some-value", nil)
			},
		},
		{
			name:                  "cache Get() throws an error",
			inputContext:          context.WithValue(context.Background(), ServiceAuthentication, validPayload),
			expectedResultContext: context.WithValue(context.Background(), ServiceAuthentication, validPayload),
			buildStubs: func(cache *mockcache.MockCache) {
				cache.EXPECT().
					Get(gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil, errors.New("some cache error"))
			},
		},
		{
			name:                  "cache Set() throws an error",
			inputContext:          context.WithValue(context.Background(), ServiceAuthentication, validPayload),
			expectedResultContext: context.WithValue(context.Background(), ServiceAuthentication, validPayload),
			buildStubs: func(cache *mockcache.MockCache) {
				cache.EXPECT().
					Get(gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil, nil)

				cache.EXPECT().
					Set(gomock.Any(), gomock.Any(), gomock.Any(), serviceAuthenticationPayloadDuration).
					Times(1).
					Return(errors.New("some cache error"))
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			crtl := gomock.NewController(t)
			defer crtl.Finish()

			cache := mockcache.NewMockCache(crtl)
			tc.buildStubs(cache)

			config := &AuthenticateServiceConfig{
				Cache: cache,
			}

			getViperStringSlice = func(_ string) []string {
				return []string{testServicePublicKeyPEM}
			}

			resultContext := authenticateService(tc.inputContext, config)
			require.Equal(t, tc.expectedResultContext, resultContext)
		})
	}
}

func TestGenerateServiceAuthenticationPayload_noPrivateKeysProvided(t *testing.T) {
	initialGetViperStringSliceFunc := getViperStringSlice
	defer func() {
		getViperStringSlice = initialGetViperStringSliceFunc
	}()

	testCases := []struct {
		name                string
		getViperStringSlice func(_ string) []string
		expectedToError     bool
	}{
		{
			name: "empty private keys slice",
			getViperStringSlice: func(_ string) []string {
				return []string{}
			},
			expectedToError: true,
		},
		{
			name: "nil private keys slice",
			getViperStringSlice: func(_ string) []string {
				return nil
			},
			expectedToError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			getViperStringSlice = tc.getViperStringSlice

			payload, err := GenerateServiceAuthenticationPayload("users")
			require.Error(t, err)
			require.Empty(t, payload)
		})
	}
}

func TestGenerateServiceAuthenticationPayload_couldNotParsePEM(t *testing.T) {
	initialGetViperStringSliceFunc := getViperStringSlice
	defer func() {
		getViperStringSlice = initialGetViperStringSliceFunc
	}()

	getViperStringSlice = func(_ string) []string {
		return []string{"invalid_PEM"}
	}

	payload, err := GenerateServiceAuthenticationPayload("users")
	require.Error(t, err)
	require.Empty(t, payload)
}

func TestGenerateServiceAuthenticationPayload_success(t *testing.T) {
	initialGetViperStringSliceFunc := getViperStringSlice
	defer func() {
		getViperStringSlice = initialGetViperStringSliceFunc
	}()

	getViperStringSlice = func(_ string) []string {
		return []string{testServicePrivateKeyPEM}
	}

	payload, err := GenerateServiceAuthenticationPayload("users")
	require.NoError(t, err)
	require.NotEmpty(t, payload)

	expectedPayloadPattern := `^users\.\d+\.[a-zA-Z0-9]{10}\.[a-zA-Z0-9+/=]+$`
	regex := regexp.MustCompile(expectedPayloadPattern)

	require.True(t, regex.MatchString(payload))
}

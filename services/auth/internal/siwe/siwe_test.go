package siwe

import (
	"context"
	"errors"
	"fmt"
	"testing"

	mockcache "github.com/spazzle-io/spazzle-api/libs/common/cache/mock"
	"github.com/spazzle-io/spazzle-api/services/auth/internal/util"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const testWalletAddress = "0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045"

func TestGenerateSIWEPayload(t *testing.T) {
	testCases := []struct {
		name          string
		domain        string
		uri           string
		chainId       uint32
		walletAddress string
		environment   string
		buildStubs    func(cache *mockcache.MockCache)
		checkResponse func(payload *Payload, err error)
	}{
		{
			name:          "success",
			domain:        "spazzle.io",
			uri:           "https://spazzle.io/login",
			chainId:       2021,
			walletAddress: testWalletAddress,
			environment:   "staging",
			buildStubs: func(cache *mockcache.MockCache) {
				cache.EXPECT().
					Set(gomock.Any(), fmt.Sprintf("%s-%s:%s", "test", prefix, testWalletAddress), gomock.Any(), expiration).
					Times(1).
					Return(nil)
			},
			checkResponse: func(payload *Payload, err error) {
				require.NoError(t, err)
				require.NotEmpty(t, payload)

				require.NotEmpty(t, payload.Nonce)
				require.NotEmpty(t, payload.Message)
				require.NotEmpty(t, payload.IssuedAt)
				require.NotEmpty(t, payload.ExpiresAt)
				require.NotEmpty(t, payload.WalletAddress)
			},
		},
		{
			name:          "invalid wallet address",
			domain:        "spazzle.io",
			uri:           "https://spazzle.io/login",
			chainId:       2021,
			walletAddress: "invalidWalletAddress",
			environment:   "staging",
			buildStubs:    func(cache *mockcache.MockCache) {},
			checkResponse: func(payload *Payload, err error) {
				require.Error(t, err)
				require.Nil(t, payload)
			},
		},
		{
			name:          "domain not allowed",
			domain:        "fakeDomain.io",
			uri:           "https://spazzle.io/login",
			chainId:       2021,
			walletAddress: testWalletAddress,
			environment:   "staging",
			buildStubs:    func(cache *mockcache.MockCache) {},
			checkResponse: func(payload *Payload, err error) {
				require.Error(t, err)
				require.Nil(t, payload)
			},
		},
		{
			name:          "chain not supported",
			domain:        "spazzle.io",
			uri:           "https://spazzle.io/login",
			chainId:       2020,
			walletAddress: testWalletAddress,
			environment:   "staging",
			buildStubs:    func(cache *mockcache.MockCache) {},
			checkResponse: func(payload *Payload, err error) {
				require.Error(t, err)
				require.Nil(t, payload)
			},
		},
		{
			name:          "environment not supported for chain",
			domain:        "spazzle.io",
			uri:           "https://spazzle.io/login",
			chainId:       2021,
			walletAddress: testWalletAddress,
			environment:   "production",
			buildStubs:    func(cache *mockcache.MockCache) {},
			checkResponse: func(payload *Payload, err error) {
				require.Error(t, err)
				require.Nil(t, payload)
			},
		},
		{
			name:          "invalid uri",
			domain:        "spazzle.io",
			uri:           "invalidUri",
			chainId:       2021,
			walletAddress: testWalletAddress,
			environment:   "staging",
			buildStubs:    func(cache *mockcache.MockCache) {},
			checkResponse: func(payload *Payload, err error) {
				require.Error(t, err)
				require.Nil(t, payload)
			},
		},
		{
			name:          "uri with www prefix",
			domain:        "spazzle.io",
			uri:           "https://www.spazzle.io/login",
			chainId:       2021,
			walletAddress: testWalletAddress,
			environment:   "staging",
			buildStubs: func(cache *mockcache.MockCache) {
				cache.EXPECT().
					Set(gomock.Any(), fmt.Sprintf("%s-%s:%s", "test", prefix, testWalletAddress), gomock.Any(), expiration).
					Times(1).
					Return(nil)
			},
			checkResponse: func(payload *Payload, err error) {
				require.NoError(t, err)
				require.NotEmpty(t, payload)
			},
		},
		{
			name:          "uri hostname does not match domain",
			domain:        "spazzle.io",
			uri:           "https://fakeDomain.io/login",
			chainId:       2021,
			walletAddress: testWalletAddress,
			environment:   "staging",
			buildStubs:    func(cache *mockcache.MockCache) {},
			checkResponse: func(payload *Payload, err error) {
				require.Error(t, err)
				require.Nil(t, payload)
			},
		},
		{
			name:          "uri using invalid schema",
			domain:        "spazzle.io",
			uri:           "http://spazzle.io/login",
			chainId:       2021,
			walletAddress: testWalletAddress,
			environment:   "staging",
			buildStubs:    func(cache *mockcache.MockCache) {},
			checkResponse: func(payload *Payload, err error) {
				require.Error(t, err)
				require.Nil(t, payload)
			},
		},
		{
			name:          "development environment bypasses schema check",
			domain:        "localhost",
			uri:           "http://localhost:3000/login",
			chainId:       2021,
			walletAddress: testWalletAddress,
			environment:   "development",
			buildStubs: func(cache *mockcache.MockCache) {
				cache.EXPECT().
					Set(gomock.Any(), fmt.Sprintf("%s-%s:%s", "test", prefix, testWalletAddress), gomock.Any(), expiration).
					Times(1).
					Return(nil)
			},
			checkResponse: func(payload *Payload, err error) {
				require.NoError(t, err)
				require.NotEmpty(t, payload)

				require.NotEmpty(t, payload.Nonce)
				require.NotEmpty(t, payload.Message)
				require.NotEmpty(t, payload.IssuedAt)
				require.NotEmpty(t, payload.ExpiresAt)
				require.NotEmpty(t, payload.WalletAddress)
			},
		},
		{
			name:          "payload cannot be cached",
			domain:        "spazzle.io",
			uri:           "https://spazzle.io/login",
			chainId:       2021,
			walletAddress: testWalletAddress,
			environment:   "development",
			buildStubs: func(cache *mockcache.MockCache) {
				cache.EXPECT().
					Set(gomock.Any(), fmt.Sprintf("%s-%s:%s", "test", prefix, testWalletAddress), gomock.Any(), expiration).
					Times(1).
					Return(errors.New("could not cache SIWE payload"))
			},
			checkResponse: func(payload *Payload, err error) {
				require.Error(t, err)
				require.Nil(t, payload)
			},
		},
	}

	siweConfig = &Config{
		Chains: []Chain{
			{
				ChainId:      2021,
				Name:         "Saigon Testnet",
				Environments: []string{"development", "staging"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := util.Config{
				ServiceName:    "test",
				Environment:    util.Environment(tc.environment),
				AllowedOrigins: []string{"http://localhost:3000", "https://spazzle.io"},
			}

			crtl := gomock.NewController(t)
			defer crtl.Finish()

			cache := mockcache.NewMockCache(crtl)
			tc.buildStubs(cache)

			payload, err := GenerateSIWEPayload(
				context.Background(), config, cache, tc.domain, tc.uri, tc.chainId, tc.walletAddress,
			)
			tc.checkResponse(payload, err)
		})
	}
}

func TestFetchSIWEMessage(t *testing.T) {
	testCases := []struct {
		name          string
		walletAddress string
		buildStubs    func(cache *mockcache.MockCache)
		checkResponse func(message string, err error)
	}{
		{
			name:          "success",
			walletAddress: testWalletAddress,
			buildStubs: func(cache *mockcache.MockCache) {
				cache.EXPECT().
					Set(gomock.Any(), fmt.Sprintf("%s-%s:%s", "test", prefix, testWalletAddress), gomock.Any(), expiration).
					Times(1).
					Return(nil)

				cache.EXPECT().
					Get(gomock.Any(), fmt.Sprintf("%s-%s:%s", "test", prefix, testWalletAddress)).
					Times(1).
					Return("some_valid_payload", nil)

				cache.EXPECT().
					Del(gomock.Any(), fmt.Sprintf("%s-%s:%s", "test", prefix, testWalletAddress)).
					Times(1).
					Return(nil)
			},
			checkResponse: func(message string, err error) {
				require.NoError(t, err)
				require.NotEmpty(t, message)
			},
		},
		{
			name:          "invalid wallet address",
			walletAddress: "invalid_wallet_address",
			buildStubs: func(cache *mockcache.MockCache) {
				cache.EXPECT().
					Set(gomock.Any(), fmt.Sprintf("%s-%s:%s", "test", prefix, testWalletAddress), gomock.Any(), expiration).
					Times(1).
					Return(nil)
			},
			checkResponse: func(message string, err error) {
				require.Error(t, err)
				require.Empty(t, message)
			},
		},
		{
			name:          "SIWE message not found in cache",
			walletAddress: testWalletAddress,
			buildStubs: func(cache *mockcache.MockCache) {
				cache.EXPECT().
					Set(gomock.Any(), fmt.Sprintf("%s-%s:%s", "test", prefix, testWalletAddress), gomock.Any(), expiration).
					Times(1).
					Return(nil)

				cache.EXPECT().
					Get(gomock.Any(), fmt.Sprintf("%s-%s:%s", "test", prefix, testWalletAddress)).
					Times(1).
					Return(nil, errors.New("SIWE message not found"))
			},
			checkResponse: func(message string, err error) {
				require.Error(t, err)
				require.Empty(t, message)
			},
		},
		{
			name:          "could not delete SIWE message from cache",
			walletAddress: testWalletAddress,
			buildStubs: func(cache *mockcache.MockCache) {
				cache.EXPECT().
					Set(gomock.Any(), fmt.Sprintf("%s-%s:%s", "test", prefix, testWalletAddress), gomock.Any(), expiration).
					Times(1).
					Return(nil)

				cache.EXPECT().
					Get(gomock.Any(), fmt.Sprintf("%s-%s:%s", "test", prefix, testWalletAddress)).
					Times(1).
					Return("some_valid_payload", nil)

				cache.EXPECT().
					Del(gomock.Any(), fmt.Sprintf("%s-%s:%s", "test", prefix, testWalletAddress)).
					Times(1).
					Return(errors.New("could not delete SIWE message from cache"))
			},
			checkResponse: func(message string, err error) {
				require.Error(t, err)
				require.Empty(t, message)
			},
		},
	}

	config := util.Config{
		ServiceName:    "test",
		Environment:    "production",
		AllowedOrigins: []string{"https://spazzle.io"},
	}

	siweConfig = &Config{
		Chains: []Chain{
			{
				ChainId:      2021,
				Name:         "Saigon Testnet",
				Environments: []string{"production"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			crtl := gomock.NewController(t)
			defer crtl.Finish()

			cache := mockcache.NewMockCache(crtl)
			tc.buildStubs(cache)

			payload, err := GenerateSIWEPayload(
				context.Background(), config, cache, "spazzle.io", "https://spazzle.io", 2021, testWalletAddress,
			)
			require.NoError(t, err)
			require.NotEmpty(t, payload)

			message, err := FetchSIWEMessage(context.Background(), config, cache, tc.walletAddress)
			tc.checkResponse(message, err)
		})
	}
}

func TestIsDomainAllowed(t *testing.T) {
	testCases := []struct {
		name           string
		domain         string
		allowedDomains []string
		isAllowed      bool
	}{
		{
			name:           "domain allowed",
			domain:         "test.com",
			allowedDomains: []string{"https://test.com"},
			isAllowed:      true,
		},
		{
			name:           "domain allowed with whitespace",
			domain:         "test.com",
			allowedDomains: []string{" https://test.com "},
			isAllowed:      true,
		},
		{
			name:           "domain allowed with www",
			domain:         "test.com",
			allowedDomains: []string{"https://www.test.com"},
			isAllowed:      true,
		},
		{
			name:           "domain not allowed",
			domain:         "fakeDomain.com",
			allowedDomains: []string{"https://test.com"},
			isAllowed:      false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			isAllowed := isDomainAllowed(tc.domain, tc.allowedDomains)
			require.Equal(t, tc.isAllowed, isAllowed)
		})
	}
}

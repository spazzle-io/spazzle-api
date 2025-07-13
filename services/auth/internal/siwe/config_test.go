package siwe

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetChain(t *testing.T) {
	testCases := []struct {
		name         string
		chainId      uint32
		environment  string
		config       Config
		isChainFound bool
	}{
		{
			name:        "success - chain found",
			chainId:     2020,
			environment: "production",
			config: Config{
				Chains: []Chain{
					{
						ChainId:      2020,
						Name:         "Ronin",
						Environments: []string{"production"},
					},
				},
			},
			isChainFound: true,
		},
		{
			name:        "failure - chain not defined",
			chainId:     2021,
			environment: "production",
			config: Config{
				Chains: []Chain{
					{
						ChainId:      2020,
						Name:         "Ronin",
						Environments: []string{"production"},
					},
				},
			},
			isChainFound: false,
		},
		{
			name:        "failure - environment not allowed for chain",
			chainId:     2021,
			environment: "production",
			config: Config{
				Chains: []Chain{
					{
						ChainId:      2021,
						Name:         "Saigon Testnet",
						Environments: []string{"development", "staging"},
					},
				},
			},
			isChainFound: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			chain := tc.config.getChain(tc.chainId, tc.environment)
			if tc.isChainFound {
				require.NotNil(t, chain)
				require.NotNil(t, chain.Name)
				require.NotNil(t, chain.Environments)
				require.Equal(t, tc.chainId, chain.ChainId)
				return
			}

			require.Nil(t, chain)
		})
	}
}

func TestLoadDefaultSIWEConfig(t *testing.T) {
	config, err := loadDefaultSIWEConfig()
	require.NoError(t, err)
	require.NotNil(t, config)

	require.Len(t, config.Chains, 2)
}

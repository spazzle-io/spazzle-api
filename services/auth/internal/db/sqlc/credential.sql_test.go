package db

import (
	"context"
	"github.com/google/uuid"
	commonUtil "github.com/spazzle-io/spazzle-api/libs/common/util"
	"github.com/stretchr/testify/require"
	"testing"
)

func createTestCredential(t *testing.T) (uuid.UUID, *commonUtil.EthereumWallet, Credential) {
	testUserId := uuid.New()

	testEthWallet, err := commonUtil.NewEthereumWallet()
	require.NoError(t, err)
	require.NotEmpty(t, testEthWallet)

	credential, err := testStore.CreateCredential(context.Background(), CreateCredentialParams{
		UserID:        testUserId,
		WalletAddress: testEthWallet.Address,
	})
	require.NoError(t, err)
	require.NotEmpty(t, credential)

	require.Equal(t, credential.UserID, testUserId)
	require.Equal(t, credential.WalletAddress, testEthWallet.Address)

	require.NotZero(t, credential.ID)
	require.NotZero(t, credential.CreatedAt)

	require.Equal(t, credential.Role, RoleUser)

	return testUserId, testEthWallet, credential
}

func TestCreateCredential(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping db test in short mode")
	}

	userId, testEthWallet, credential := createTestCredential(t)
	require.NotEmpty(t, userId)
	require.NotEmpty(t, testEthWallet)
	require.NotEmpty(t, credential)
}

func TestGetCredentialByWalletAddress(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping db test in short mode")
	}

	userId, testEthWallet, credential := createTestCredential(t)
	require.NotEmpty(t, userId)
	require.NotEmpty(t, testEthWallet)
	require.NotEmpty(t, credential)

	testCases := []struct {
		name              string
		walletAddress     string
		isCredentialFound bool
	}{
		{
			name:              "Success",
			walletAddress:     testEthWallet.Address,
			isCredentialFound: true,
		},
		{
			name:              "Credential not found",
			walletAddress:     "0x0000000000000000000000000000000000000000",
			isCredentialFound: false,
		},
	}

	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			fetchedCredential, err := testStore.GetCredentialByWalletAddress(context.Background(), tc.walletAddress)

			if tc.isCredentialFound {
				require.NoError(t, err)
				require.NotEmpty(t, fetchedCredential)
				require.Equal(t, credential, fetchedCredential)
			} else {
				require.Equal(t, err, RecordNotFoundError)
				require.Equal(t, fetchedCredential, Credential{})
			}
		})
	}
}

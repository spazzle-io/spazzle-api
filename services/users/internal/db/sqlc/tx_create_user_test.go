package db

import (
	"context"
	"testing"
	"time"

	commonUtil "github.com/spazzle-io/spazzle-api/libs/common/util"
	"github.com/stretchr/testify/require"
)

func TestCreateUserTx(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping db test in short mode")
	}

	testCases := []struct {
		name        string
		buildParams func(
			t *testing.T,
			wallet *commonUtil.EthereumWallet,
			isAfterCreateCalled *bool) CreateUserTxParams
		checkResult func(
			t *testing.T,
			wallet *commonUtil.EthereumWallet,
			isAfterCreateCalled *bool,
			txResult CreateUserTxResult,
			err error)
	}{
		{
			name: "success",
			buildParams: func(t *testing.T, wallet *commonUtil.EthereumWallet, isAfterCreateCalled *bool) CreateUserTxParams {
				return CreateUserTxParams{
					WalletAddress: wallet.Address,
					AfterCreate: func(user User) error {
						*isAfterCreateCalled = true
						return nil
					},
				}
			},
			checkResult: func(t *testing.T, wallet *commonUtil.EthereumWallet, isAfterCreateCalled *bool, txResult CreateUserTxResult, err error) {
				require.Equal(t, wallet.Address, txResult.User.WalletAddress)
				require.Empty(t, txResult.User.GamerTag.String)
				require.NotZero(t, txResult.User.CreatedAt)
				require.WithinDuration(t, time.Now().UTC(), txResult.User.CreatedAt, time.Second)

				require.NoError(t, err)

				require.True(t, *isAfterCreateCalled)
			},
		},
		{
			name: "user already exists",
			buildParams: func(t *testing.T, wallet *commonUtil.EthereumWallet, isAfterCreateCalled *bool) CreateUserTxParams {
				user, err := testStore.CreateUser(context.Background(), wallet.Address)
				require.NoError(t, err)
				require.NotEmpty(t, user)

				return CreateUserTxParams{
					WalletAddress: wallet.Address,
					AfterCreate: func(user User) error {
						*isAfterCreateCalled = true
						return nil
					},
				}
			},
			checkResult: func(t *testing.T, wallet *commonUtil.EthereumWallet, isAfterCreateCalled *bool, txResult CreateUserTxResult, err error) {
				require.Empty(t, txResult.User)
				require.Error(t, err)
				require.Equal(t, ErrUserAlreadyExists, err)

				require.False(t, *isAfterCreateCalled)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			isAfterCreateCalled := false

			wallet, err := commonUtil.NewEthereumWallet()
			require.NoError(t, err)
			require.NotEmpty(t, wallet)

			params := tc.buildParams(t, wallet, &isAfterCreateCalled)
			txResult, err := testStore.CreateUserTx(context.Background(), params)
			tc.checkResult(t, wallet, &isAfterCreateCalled, txResult, err)
		})
	}
}

package db

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	commonUtil "github.com/spazzle-io/spazzle-api/libs/common/util"
	"github.com/stretchr/testify/require"
)

func TestCreateCredentialTx(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping db test in short mode")
	}

	testCases := []struct {
		name        string
		buildParams func(userId uuid.UUID, wallet *commonUtil.EthereumWallet, isAfterCreateCalled *bool) *CreateCredentialTxParams
		checkResult func(t *testing.T, isAfterCreateCalled *bool, userId uuid.UUID, wallet *commonUtil.EthereumWallet, txResult CreateCredentialTxResult, err error)
	}{
		{
			name: "success",
			buildParams: func(userId uuid.UUID, wallet *commonUtil.EthereumWallet, isAfterCreateCalled *bool) *CreateCredentialTxParams {
				return &CreateCredentialTxParams{
					CreateCredentialParams: CreateCredentialParams{
						UserID:        userId,
						WalletAddress: wallet.Address,
					},
					AfterCreate: func(credential Credential) error {
						*isAfterCreateCalled = true
						return nil
					},
				}
			},
			checkResult: func(t *testing.T, isAfterCreateCalled *bool, userId uuid.UUID, wallet *commonUtil.EthereumWallet, txResult CreateCredentialTxResult, err error) {
				require.True(t, *isAfterCreateCalled)

				require.NoError(t, err)

				require.Equal(t, userId, txResult.Credential.UserID)
				require.Equal(t, wallet.Address, txResult.Credential.WalletAddress)
				require.Equal(t, RoleUser, txResult.Credential.Role)
				require.NotZero(t, txResult.Credential.ID)
				require.NotZero(t, txResult.Credential.CreatedAt)
				require.WithinDuration(t, time.Now().UTC(), txResult.Credential.CreatedAt, time.Second)
			},
		},
		{
			name: "credential with similar user ID exists",
			buildParams: func(userId uuid.UUID, wallet *commonUtil.EthereumWallet, isAfterCreateCalled *bool) *CreateCredentialTxParams {
				otherWallet, err := commonUtil.NewEthereumWallet()
				require.NoError(t, err)
				require.NotEmpty(t, otherWallet)

				credential, err := testStore.CreateCredential(context.Background(), CreateCredentialParams{
					UserID:        userId,
					WalletAddress: otherWallet.Address,
				})
				require.NoError(t, err)
				require.NotEmpty(t, credential)

				return &CreateCredentialTxParams{
					CreateCredentialParams: CreateCredentialParams{
						UserID:        userId,
						WalletAddress: wallet.Address,
					},
					AfterCreate: func(credential Credential) error {
						*isAfterCreateCalled = true
						return nil
					},
				}
			},
			checkResult: func(t *testing.T, isAfterCreateCalled *bool, userId uuid.UUID, wallet *commonUtil.EthereumWallet, txResult CreateCredentialTxResult, err error) {
				require.False(t, *isAfterCreateCalled)

				require.Error(t, err)
				require.Equal(t, ErrCredentialAlreadyExists, err)
				require.Empty(t, txResult)
			},
		},
		{
			name: "credential with similar wallet address exists",
			buildParams: func(userId uuid.UUID, wallet *commonUtil.EthereumWallet, isAfterCreateCalled *bool) *CreateCredentialTxParams {
				otherUserId := uuid.New()

				credential, err := testStore.CreateCredential(context.Background(), CreateCredentialParams{
					UserID:        otherUserId,
					WalletAddress: wallet.Address,
				})
				require.NoError(t, err)
				require.NotEmpty(t, credential)

				return &CreateCredentialTxParams{
					CreateCredentialParams: CreateCredentialParams{
						UserID:        userId,
						WalletAddress: wallet.Address,
					},
					AfterCreate: func(credential Credential) error {
						*isAfterCreateCalled = true
						return nil
					},
				}
			},
			checkResult: func(t *testing.T, isAfterCreateCalled *bool, userId uuid.UUID, wallet *commonUtil.EthereumWallet, txResult CreateCredentialTxResult, err error) {
				require.False(t, *isAfterCreateCalled)

				require.Error(t, err)
				require.Equal(t, ErrCredentialAlreadyExists, err)
				require.Empty(t, txResult)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			isAfterCreateCalled := false

			wallet, err := commonUtil.NewEthereumWallet()
			require.NoError(t, err)
			require.NotEmpty(t, wallet)

			userId := uuid.New()

			params := tc.buildParams(userId, wallet, &isAfterCreateCalled)
			txResult, err := testStore.CreateCredentialTx(context.Background(), *params)
			tc.checkResult(t, &isAfterCreateCalled, userId, wallet, txResult, err)
		})
	}
}

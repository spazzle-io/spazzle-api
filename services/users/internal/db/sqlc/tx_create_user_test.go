package db

import (
	"context"
	"testing"
	"time"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/jackc/pgx/v5/pgtype"
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
			gamerTag string,
			isAfterCreateCalled *bool) CreateUserTxParams
		checkResult func(
			t *testing.T,
			wallet *commonUtil.EthereumWallet,
			gamerTag string,
			isAfterCreateCalled *bool,
			txResult CreateUserTxResult,
			err error)
	}{
		{
			name: "success",
			buildParams: func(t *testing.T, wallet *commonUtil.EthereumWallet, gamerTag string, isAfterCreateCalled *bool) CreateUserTxParams {
				return CreateUserTxParams{
					CreateUserParams: CreateUserParams{
						WalletAddress: wallet.Address,
						GamerTag: pgtype.Text{
							String: gamerTag,
							Valid:  true,
						},
					},
					AfterCreate: func() error {
						*isAfterCreateCalled = true
						return nil
					},
				}
			},
			checkResult: func(t *testing.T, wallet *commonUtil.EthereumWallet, gamerTag string, isAfterCreateCalled *bool, txResult CreateUserTxResult, err error) {
				require.Equal(t, wallet.Address, txResult.User.WalletAddress)
				require.Equal(t, gamerTag, txResult.User.GamerTag.String)
				require.NotZero(t, txResult.User.CreatedAt)
				require.WithinDuration(t, time.Now().UTC(), txResult.User.CreatedAt, time.Second)

				require.NoError(t, err)

				require.True(t, *isAfterCreateCalled)
			},
		},
		{
			name: "user already exists",
			buildParams: func(t *testing.T, wallet *commonUtil.EthereumWallet, gamerTag string, isAfterCreateCalled *bool) CreateUserTxParams {
				user, err := testStore.CreateUser(context.Background(), CreateUserParams{
					WalletAddress: wallet.Address,
					GamerTag: pgtype.Text{
						String: gamerTag,
						Valid:  true,
					},
				})
				require.NoError(t, err)
				require.NotEmpty(t, user)

				return CreateUserTxParams{
					CreateUserParams: CreateUserParams{
						WalletAddress: wallet.Address,
						GamerTag: pgtype.Text{
							String: gamerTag,
							Valid:  true,
						},
					},
					AfterCreate: func() error {
						*isAfterCreateCalled = true
						return nil
					},
				}
			},
			checkResult: func(t *testing.T, wallet *commonUtil.EthereumWallet, gamerTag string, isAfterCreateCalled *bool, txResult CreateUserTxResult, err error) {
				require.Empty(t, txResult.User)
				require.Error(t, err)
				require.Equal(t, ErrUserAlreadyExists, err)

				require.False(t, *isAfterCreateCalled)
			},
		},
		{
			name: "gamer tag already in use",
			buildParams: func(t *testing.T, wallet *commonUtil.EthereumWallet, gamerTag string, isAfterCreateCalled *bool) CreateUserTxParams {
				otherWallet, err := commonUtil.NewEthereumWallet()
				require.NoError(t, err)
				require.NotEmpty(t, otherWallet)

				user, err := testStore.CreateUser(context.Background(), CreateUserParams{
					WalletAddress: otherWallet.Address,
					GamerTag: pgtype.Text{
						String: gamerTag,
						Valid:  true,
					},
				})
				require.NoError(t, err)
				require.NotEmpty(t, user)

				return CreateUserTxParams{
					CreateUserParams: CreateUserParams{
						WalletAddress: wallet.Address,
						GamerTag: pgtype.Text{
							String: gamerTag,
							Valid:  true,
						},
					},
					AfterCreate: func() error {
						*isAfterCreateCalled = true
						return nil
					},
				}
			},
			checkResult: func(t *testing.T, wallet *commonUtil.EthereumWallet, gamerTag string, isAfterCreateCalled *bool, txResult CreateUserTxResult, err error) {
				require.Empty(t, txResult.User)
				require.Error(t, err)
				require.Equal(t, ErrGamerTagAlreadyInUse, err)

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

			gamerTag := gofakeit.Gamertag()
			require.NotEmpty(t, gamerTag)

			params := tc.buildParams(t, wallet, gamerTag, &isAfterCreateCalled)
			txResult, err := testStore.CreateUserTx(context.Background(), params)
			tc.checkResult(t, wallet, gamerTag, &isAfterCreateCalled, txResult, err)
		})
	}
}

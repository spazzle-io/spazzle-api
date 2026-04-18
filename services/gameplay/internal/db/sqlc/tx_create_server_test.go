package db

import (
	"context"
	"fmt"
	"math/big"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/ethereum/go-ethereum/common"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	commonUtil "github.com/spazzle-io/spazzle-api/libs/common/util"
	"github.com/stretchr/testify/require"
)

func TestCreateServerTx(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping db test in short mode")
	}

	randStr, err := commonUtil.GenerateRandomAlphanumericString(5)
	require.NoError(t, err)
	require.NotEmpty(t, randStr)

	serverWallet, err := commonUtil.NewEthereumWallet()
	require.NoError(t, err)
	require.NotEmpty(t, serverWallet)

	serverOwnerWallet, err := commonUtil.NewEthereumWallet()
	require.NoError(t, err)
	require.NotEmpty(t, serverOwnerWallet)

	params := CreateServerTxParams{
		CreateServerParams: CreateServerParams{
			ID:            uuid.New(),
			Name:          fmt.Sprintf("%s-%s", gofakeit.BeerAlcohol(), randStr),
			OwnerID:       uuid.New(),
			ServerAddress: serverWallet.Address,
			StakePerGame: pgtype.Numeric{
				Int:   big.NewInt(100),
				Valid: true,
			},
		},
		ServerOwnerAddress: common.HexToAddress(serverOwnerWallet.Address),
		AfterCreate: func(treasury ServerTreasury) error {
			return nil
		},
	}

	result, err := testStore.CreateServerTx(context.Background(), params)
	require.NoError(t, err)

	require.NotEmpty(t, result)
	require.NotEmpty(t, result.Server)
	require.NotEmpty(t, result.Treasury)

	require.Equal(t, result.Server.ID, result.Treasury.ServerID)
	require.Equal(t, serverOwnerWallet.Address, result.Treasury.Owner)
	require.Equal(t, TreasuryStatusPending, result.Treasury.Status)
	require.Equal(t, result.Server.ServerAddress, result.Treasury.Address)
}

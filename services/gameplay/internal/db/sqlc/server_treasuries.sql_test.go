package db

import (
	"context"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/ethereum/go-ethereum/common"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	commonUtil "github.com/spazzle-io/spazzle-api/libs/common/util"
	"github.com/stretchr/testify/require"
)

func createTestTreasury(t *testing.T, owner common.Address) ServerTreasury {
	t.Helper()

	serverWallet, err := commonUtil.NewEthereumWallet()
	require.NoError(t, err)
	require.NotEmpty(t, serverWallet)

	randStr, err := commonUtil.GenerateRandomAlphanumericString(4)
	require.NoError(t, err)
	require.NotEmpty(t, randStr)

	createServerParams := CreateServerParams{
		ID:            uuid.New(),
		Name:          fmt.Sprintf("%s_%s", gofakeit.PetName(), randStr),
		OwnerID:       uuid.New(),
		ServerAddress: serverWallet.Address,
		StakePerGame: pgtype.Numeric{
			Int:   big.NewInt(100),
			Valid: true,
		},
		NumRoundsPerGame:  10,
		RoundDurationSecs: 60,
		NumDrawingOptions: 3,
	}

	server, err := testStore.CreateServer(context.Background(), createServerParams)
	require.NoError(t, err)
	require.NotEmpty(t, server)

	createTreasuryParams := CreateTreasuryParams{
		ServerID: server.ID,
		Owner:    owner.String(),
		Address:  serverWallet.Address,
	}

	treasury, err := testStore.CreateTreasury(context.Background(), createTreasuryParams)
	require.NoError(t, err)
	require.NotEmpty(t, treasury)

	require.Equal(t, server.ID, treasury.ServerID)
	require.Equal(t, owner.String(), treasury.Owner)
	require.Equal(t, serverWallet.Address, treasury.Address)
	require.Equal(t, TreasuryStatusPending, treasury.Status)
	require.WithinDuration(t, time.Now().UTC(), treasury.CreatedAt, time.Second)
	require.WithinDuration(t, time.Now().UTC(), treasury.UpdatedAt, time.Second)

	return treasury
}

func TestCreateTreasury(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping db test in short mode")
	}

	owner := common.HexToAddress("0x2222222222222222222222222222222222222222")

	treasury := createTestTreasury(t, owner)
	require.NotEmpty(t, treasury)
}

func TestGetTreasury(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping db test in short mode")
	}

	owner := common.HexToAddress("0x222222222222222222222222222222222222")

	treasury := createTestTreasury(t, owner)
	require.NotEmpty(t, treasury)

	got, err := testStore.GetTreasury(context.Background(), treasury.Address)
	require.NoError(t, err)
	require.Equal(t, treasury.Address, got.Address)
}

func TestMarkTreasuryDeploying(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping db test in short mode")
	}

	owner := common.HexToAddress("0x222222222222222222222222222222222222")

	treasury := createTestTreasury(t, owner)
	require.NotEmpty(t, treasury)

	require.Equal(t, TreasuryStatusPending, treasury.Status)

	got, err := testStore.MarkTreasuryDeploying(context.Background(), treasury.Address)
	require.NoError(t, err)
	require.Equal(t, TreasuryStatusDeploying, got.Status)
	require.WithinDuration(t, time.Now().UTC(), got.UpdatedAt, time.Second)
}

func TestMarkTreasuryDeployed(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping db test in short mode")
	}

	owner := common.HexToAddress("0x222222222222222222222222222222222222")

	treasury := createTestTreasury(t, owner)
	require.NotEmpty(t, treasury)

	require.Equal(t, TreasuryStatusPending, treasury.Status)

	txHash := pgtype.Text{
		String: "123",
		Valid:  true,
	}

	blockNumber := pgtype.Int8{
		Int64: 321,
		Valid: true,
	}

	gasUsed := pgtype.Int8{
		Int64: 231,
		Valid: true,
	}

	got, err := testStore.MarkTreasuryDeployed(context.Background(), MarkTreasuryDeployedParams{
		Address:     treasury.Address,
		TxHash:      txHash,
		BlockNumber: blockNumber,
		GasUsed:     gasUsed,
	})
	require.NoError(t, err)
	require.Equal(t, TreasuryStatusDeployed, got.Status)
	require.Equal(t, txHash, got.TxHash)
	require.Equal(t, blockNumber, got.BlockNumber)
	require.Equal(t, gasUsed, got.GasUsed)
	require.WithinDuration(t, time.Now().UTC(), got.DeployedAt.Time, time.Second)
	require.WithinDuration(t, time.Now().UTC(), got.UpdatedAt, time.Second)
}

func TestRecoverDeployedTreasury(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping db test in short mode")
	}

	owner := common.HexToAddress("0x222222222222222222222222222222222222")

	treasury := createTestTreasury(t, owner)
	require.NotEmpty(t, treasury)

	require.Equal(t, TreasuryStatusPending, treasury.Status)

	got, err := testStore.RecoverDeployedTreasury(context.Background(), treasury.Address)
	require.NoError(t, err)
	require.Equal(t, TreasuryStatusDeployed, got.Status)
	require.WithinDuration(t, time.Now().UTC(), got.UpdatedAt, time.Second)
}

func TestMarkTreasuryFailed(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping db test in short mode")
	}

	owner := common.HexToAddress("0x222222222222222222222222222222222222")

	treasury := createTestTreasury(t, owner)
	require.NotEmpty(t, treasury)

	require.Equal(t, TreasuryStatusPending, treasury.Status)

	got, err := testStore.MarkTreasuryFailed(context.Background(), treasury.Address)
	require.NoError(t, err)
	require.Equal(t, TreasuryStatusFailed, got.Status)
	require.WithinDuration(t, time.Now().UTC(), got.UpdatedAt, time.Second)
}

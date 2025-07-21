package db

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/jackc/pgx/v5/pgtype"
	commonUtil "github.com/spazzle-io/spazzle-api/libs/common/util"
	"github.com/stretchr/testify/require"
)

func createTestUser(t *testing.T) User {
	wallet, err := commonUtil.NewEthereumWallet()
	require.NoError(t, err)
	require.NotEmpty(t, wallet)

	params := CreateUserParams{
		WalletAddress: wallet.Address,
		GamerTag: pgtype.Text{
			String: gofakeit.Gamertag(),
			Valid:  true,
		},
	}

	user, err := testStore.CreateUser(context.Background(), params)
	require.NoError(t, err)
	require.NotEmpty(t, user)

	require.Equal(t, params.WalletAddress, user.WalletAddress)
	require.Equal(t, params.GamerTag, user.GamerTag)
	require.NotZero(t, user.ID)
	require.WithinDuration(t, time.Now().UTC(), user.CreatedAt, time.Second)
	require.NotZero(t, user.CreatedAt)

	require.Empty(t, user.EnsName)
	require.Empty(t, user.EnsAvatarUri)
	require.Empty(t, user.EnsImageUrl)
	require.Empty(t, user.EnsLastResolvedAt)

	return user
}

func TestCreateUser(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping db test in short mode")
	}

	user := createTestUser(t)
	require.NotEmpty(t, user)
}

func TestGetUserById(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping db test in short mode")
	}

	user := createTestUser(t)
	require.NotEmpty(t, user)

	fetchedUser, err := testStore.GetUserById(context.Background(), user.ID)
	require.NoError(t, err)
	require.Equal(t, user, fetchedUser)
}

func TestGetTotalUserCount(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping db test in short mode")
	}

	initialUserCount, err := testStore.GetTotalUserCount(context.Background())
	require.NoError(t, err)

	numAdditionalUsers := 6
	for i := 0; i < numAdditionalUsers; i++ {
		user := createTestUser(t)
		require.NotEmpty(t, user)
	}

	finalUserCount, err := testStore.GetTotalUserCount(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, finalUserCount)

	expectedUserCount := initialUserCount + int64(numAdditionalUsers)
	require.Equal(t, expectedUserCount, finalUserCount)
}

func TestListUsers(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping db test in short mode")
	}

	var recentWalletAddresses []string
	numUsersToCreate := 4

	for i := 0; i < numUsersToCreate; i++ {
		user := createTestUser(t)
		require.NotEmpty(t, user)
		recentWalletAddresses = append(recentWalletAddresses, user.WalletAddress)
	}

	params := ListUsersParams{
		Limit:  int32(numUsersToCreate) - 1,
		Offset: 1,
	}
	recentlyCreatedUsers, err := testStore.ListUsers(context.Background(), params)
	require.NoError(t, err)
	require.NotEmpty(t, recentlyCreatedUsers)

	for idx, user := range recentlyCreatedUsers {
		require.Equal(t, recentWalletAddresses[len(recentWalletAddresses)-idx-2], user.WalletAddress)
	}
}

func TestUpdateUser(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping db test in short mode")
	}

	user := createTestUser(t)
	require.NotEmpty(t, user)

	ensName := fmt.Sprintf("%s.eth", gofakeit.Gamertag())
	ensAvatarUri := "eip155:1/erc721:0x4bb08998a697d0db666783ba5b56e85b33ba262f/7967"
	ensImageUrl := "https://i2.seadn.io/ethereum/0x4bb08998a697d0db666783ba5b56e85b33ba262f/1d219876a9abcf6bb2020ea6dcf7ad6f.png"
	ensLastResolvedAt := time.Now().UTC()

	expectedUpdatedGamerTag := fmt.Sprintf("%s--updated", user.GamerTag.String)

	params := UpdateUserParams{
		UserID: user.ID,
		GamerTag: pgtype.Text{
			String: expectedUpdatedGamerTag,
			Valid:  true,
		},
		EnsName: pgtype.Text{
			String: ensName,
			Valid:  true,
		},
		EnsAvatarUri: pgtype.Text{
			String: ensAvatarUri,
			Valid:  true,
		},
		EnsImageUrl: pgtype.Text{
			String: ensImageUrl,
			Valid:  true,
		},
		EnsLastResolvedAt: pgtype.Timestamptz{
			Time:  ensLastResolvedAt,
			Valid: true,
		},
	}
	updatedUser, err := testStore.UpdateUser(context.Background(), params)
	require.NoError(t, err)
	require.NotEmpty(t, updatedUser)

	require.Equal(t, expectedUpdatedGamerTag, updatedUser.GamerTag.String)
	require.Equal(t, user.WalletAddress, updatedUser.WalletAddress)
	require.Equal(t, updatedUser.EnsName.String, ensName)
	require.Equal(t, updatedUser.EnsAvatarUri.String, ensAvatarUri)
	require.Equal(t, updatedUser.EnsImageUrl.String, ensImageUrl)
	require.WithinDuration(t, updatedUser.EnsLastResolvedAt.Time, ensLastResolvedAt, time.Second)
	require.WithinDuration(t, user.CreatedAt, updatedUser.CreatedAt, time.Second)
}

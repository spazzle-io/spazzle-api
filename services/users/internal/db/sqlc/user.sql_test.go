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

	user, err := testStore.CreateUser(context.Background(), wallet.Address)
	require.NoError(t, err)
	require.NotEmpty(t, user)

	require.Equal(t, wallet.Address, user.WalletAddress)
	require.NotZero(t, user.ID)
	require.WithinDuration(t, time.Now().UTC(), user.CreatedAt, time.Second)
	require.NotZero(t, user.CreatedAt)

	return user
}

func TestCreateUser(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping db test in short mode")
	}

	user := createTestUser(t)
	require.NotEmpty(t, user)
}

func TestGetUserByWalletAddress(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping db test in short mode")
	}

	user := createTestUser(t)
	require.NotEmpty(t, user)

	fetchedUser, err := testStore.GetUserByWalletAddress(context.Background(), user.WalletAddress)
	require.NoError(t, err)
	require.Equal(t, user, fetchedUser)
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
	numUsersToCreate := 5
	lastSeenUserIdx := 0

	for i := 0; i < numUsersToCreate; i++ {
		user := createTestUser(t)
		require.NotEmpty(t, user)
		recentWalletAddresses = append(recentWalletAddresses, user.WalletAddress)
	}

	firstPageParams := ListUsersParams{
		PageSize: 2,
	}
	firstPageUsers, err := testStore.ListUsers(context.Background(), firstPageParams)
	require.NoError(t, err)
	require.NotEmpty(t, firstPageUsers)
	require.Len(t, firstPageUsers, 2)

	for _, user := range firstPageUsers {
		require.Equal(t, recentWalletAddresses[len(recentWalletAddresses)-lastSeenUserIdx-1], user.WalletAddress)
		lastSeenUserIdx += 1
	}

	lastPageParams := ListUsersParams{
		PageSize: int32(numUsersToCreate),
		AfterCreatedAt: pgtype.Timestamptz{
			Time:  firstPageUsers[len(firstPageUsers)-1].CreatedAt,
			Valid: true,
		},
		AfterID: pgtype.UUID{
			Bytes: firstPageUsers[len(firstPageUsers)-1].ID,
			Valid: true,
		},
	}
	lastPageUsers, err := testStore.ListUsers(context.Background(), lastPageParams)
	expectedNumLastPageUsers := numUsersToCreate - lastSeenUserIdx
	require.NoError(t, err)
	require.NotEmpty(t, lastPageUsers)
	require.Greater(t, len(lastPageUsers), expectedNumLastPageUsers)
	for i := 0; i < expectedNumLastPageUsers; i++ {
		user := lastPageUsers[i]
		require.Equal(t, recentWalletAddresses[len(recentWalletAddresses)-lastSeenUserIdx-1], user.WalletAddress)
		lastSeenUserIdx += 1
	}
}

func TestUpdateUser(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping db test in short mode")
	}

	user := createTestUser(t)
	require.NotEmpty(t, user)

	gamerTag := gofakeit.Gamertag()
	require.NotEmpty(t, gamerTag)

	expectedUpdatedGamerTag := fmt.Sprintf("%s--updated", gamerTag)

	params := UpdateUserParams{
		UserID: user.ID,
		GamerTag: pgtype.Text{
			String: expectedUpdatedGamerTag,
			Valid:  true,
		},
	}
	updatedUser, err := testStore.UpdateUser(context.Background(), params)
	require.NoError(t, err)
	require.NotEmpty(t, updatedUser)

	require.Equal(t, expectedUpdatedGamerTag, updatedUser.GamerTag.String)
	require.Equal(t, user.WalletAddress, updatedUser.WalletAddress)
	require.WithinDuration(t, user.CreatedAt, updatedUser.CreatedAt, time.Second)
}

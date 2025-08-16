package db

import (
	"context"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	commonUtil "github.com/spazzle-io/spazzle-api/libs/common/util"
	"github.com/stretchr/testify/require"
)

func createTestServer(t *testing.T, userId uuid.UUID) Server {
	serverWallet, err := commonUtil.NewEthereumWallet()
	require.NoError(t, err)
	require.NotEmpty(t, serverWallet)

	randStr, err := commonUtil.GenerateRandomAlphanumericString(4)
	require.NoError(t, err)
	require.NotEmpty(t, randStr)

	stakePerGameWeiStr := "2000000000000000"
	stakePerGame, err := ParseWeiStrToBigInt(stakePerGameWeiStr)
	require.NoError(t, err)
	require.NotEmpty(t, stakePerGame)

	params := CreateServerParams{
		Name:          fmt.Sprintf("%s_%s", gofakeit.PetName(), randStr),
		OwnerID:       userId,
		ServerAddress: serverWallet.Address,
		StakePerGame: pgtype.Numeric{
			Int:   stakePerGame,
			Valid: true,
		},
		NumRoundsPerGame:  10,
		RoundDurationSecs: 1.5 * 60,
		NumDrawingOptions: 3,
	}

	server, err := testStore.CreateServer(context.Background(), params)
	require.NoError(t, err)
	require.NotEmpty(t, server)

	require.NotEmpty(t, server.ID)

	require.NotEmpty(t, server.Name)
	require.Equal(t, params.Name, server.Name)

	require.NotEmpty(t, server.OwnerID)
	require.Equal(t, params.OwnerID, server.OwnerID)

	require.Empty(t, server.NumAdmins)

	require.Empty(t, server.NumCustomWords)

	require.NotEmpty(t, server.ServerAddress)
	require.Equal(t, params.ServerAddress, server.ServerAddress)

	gotStakePerGameStr, err := ParseDBNumericWeiToStr(server.StakePerGame)
	require.NoError(t, err)
	require.NotEmpty(t, gotStakePerGameStr)
	require.Equal(t, stakePerGameWeiStr, gotStakePerGameStr)

	require.NotEmpty(t, server.NumRoundsPerGame)
	require.Equal(t, params.NumRoundsPerGame, server.NumRoundsPerGame)

	require.NotEmpty(t, server.RoundDurationSecs)
	require.Equal(t, params.RoundDurationSecs, server.RoundDurationSecs)

	require.NotEmpty(t, server.NumDrawingOptions)
	require.Equal(t, params.NumDrawingOptions, server.NumDrawingOptions)

	require.False(t, server.IsArchived)
	require.Empty(t, server.ArchivedAt)

	require.WithinDuration(t, time.Now().UTC(), server.CreatedAt, time.Second)

	return server
}

func TestCreateServer(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping db test in short mode")
	}

	server := createTestServer(t, uuid.New())
	require.NotEmpty(t, server)
}

func TestCreateServer_UniqueName(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping db test in short mode")
	}

	// create a new server
	server := createTestServer(t, uuid.New())
	require.NotEmpty(t, server)
	require.False(t, server.IsArchived)

	// archive the created server
	updatedServer, err := testStore.UpdateServer(context.Background(), UpdateServerParams{
		ServerID: server.ID,
		IsArchived: pgtype.Bool{
			Bool:  true,
			Valid: true,
		},
		ArchivedAt: pgtype.Timestamptz{
			Time:  time.Now().UTC(),
			Valid: true,
		},
	})
	require.NoError(t, err)
	require.Equal(t, server.ID, updatedServer.ID)
	require.True(t, updatedServer.IsArchived)

	// create a new server with the same name as the previous one - should succeed because it's archived
	createServerParams := CreateServerParams{
		Name:          server.Name,
		OwnerID:       server.OwnerID,
		ServerAddress: server.ServerAddress,
		StakePerGame: pgtype.Numeric{
			Int:   big.NewInt(20),
			Valid: true,
		},
		NumRoundsPerGame:  10,
		RoundDurationSecs: 1.5 * 60,
		NumDrawingOptions: 3,
	}
	newServer, err := testStore.CreateServer(context.Background(), createServerParams)
	require.NoError(t, err)
	require.NotEmpty(t, newServer)

	// attempt to create another server with the same name - should fail
	anotherNewServer, err := testStore.CreateServer(context.Background(), createServerParams)
	require.Error(t, err)
	require.ErrorContains(t, err, "SQLSTATE 23505")
	require.ErrorContains(t, err, "servers_name_unique_unarchived_idx")
	require.Empty(t, anotherNewServer)
}

func TestGetServerById(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping db test in short mode")
	}

	server := createTestServer(t, uuid.New())
	require.NotEmpty(t, server)

	fetchedServer, err := testStore.GetServerById(context.Background(), server.ID)
	require.NoError(t, err)
	require.NotEmpty(t, fetchedServer)

	require.Equal(t, server.ID, fetchedServer.ID)
}

func TestGetServerByName(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping db test in short mode")
	}

	server := createTestServer(t, uuid.New())
	require.NotEmpty(t, server)

	fetchedServer, err := testStore.GetServerByName(context.Background(), server.Name)
	require.NoError(t, err)
	require.NotEmpty(t, fetchedServer)

	require.Equal(t, server.ID, fetchedServer.ID)
	require.Equal(t, server.Name, fetchedServer.Name)
}

func TestGetTotalUserServersCount(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping db test in short mode")
	}

	userId := uuid.New()

	initialUserServersCount, err := testStore.GetTotalUserServersCount(context.Background(), userId)
	require.NoError(t, err)
	require.NotNil(t, initialUserServersCount)

	numAdditionalServers := 5
	for i := 0; i < numAdditionalServers; i++ {
		server := createTestServer(t, userId)
		require.NotEmpty(t, server)
	}

	finalUserServersCount, err := testStore.GetTotalUserServersCount(context.Background(), userId)
	require.NoError(t, err)
	require.NotEmpty(t, finalUserServersCount)

	expectedUserServersCount := initialUserServersCount + int64(numAdditionalServers)
	require.Equal(t, expectedUserServersCount, finalUserServersCount)
}

func TestGetServerUserPermissions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping db test in short mode")
	}

	userId := uuid.New()

	server := createTestServer(t, userId)
	require.NotEmpty(t, server)

	testCases := []struct {
		name                           string
		userId                         uuid.UUID
		expectedIsOwner                bool
		expectedIsAdmin                bool
		expectedHasElevatedPermissions bool
		createEntities                 func(t *testing.T, userId uuid.UUID)
	}{
		{
			name:                           "is owner",
			userId:                         userId,
			expectedIsOwner:                true,
			expectedIsAdmin:                false,
			expectedHasElevatedPermissions: true,
			createEntities: func(t *testing.T, userId uuid.UUID) {
				admin := createTestServerAdmin(t, server.ID, uuid.New())
				require.NotEmpty(t, admin)
			},
		},
		{
			name:                           "is admin",
			userId:                         uuid.New(),
			expectedIsOwner:                false,
			expectedIsAdmin:                true,
			expectedHasElevatedPermissions: true,
			createEntities: func(t *testing.T, userId uuid.UUID) {
				admin := createTestServerAdmin(t, server.ID, userId)
				require.NotEmpty(t, admin)
			},
		},
		{
			name:                           "both owner and admin",
			userId:                         userId,
			expectedIsOwner:                true,
			expectedIsAdmin:                true,
			expectedHasElevatedPermissions: true,
			createEntities: func(t *testing.T, userId uuid.UUID) {
				admin := createTestServerAdmin(t, server.ID, userId)
				require.NotEmpty(t, admin)
			},
		},
		{
			name:                           "user has no server permissions",
			userId:                         uuid.New(),
			expectedIsOwner:                false,
			expectedIsAdmin:                false,
			expectedHasElevatedPermissions: false,
			createEntities: func(t *testing.T, userId uuid.UUID) {
				admin := createTestServerAdmin(t, server.ID, uuid.New())
				require.NotEmpty(t, admin)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.createEntities(t, tc.userId)

			params := GetServerUserPermissionsParams{
				UserID:   tc.userId,
				ServerID: server.ID,
			}
			res, err := testStore.GetServerUserPermissions(context.Background(), params)
			require.NoError(t, err)

			require.Equal(t, tc.expectedIsOwner, res.IsOwner)
			require.Equal(t, tc.expectedIsAdmin, res.IsAdmin)
			require.Equal(t, tc.expectedHasElevatedPermissions, res.HasElevatedPermissions)
		})
	}
}

func TestListUserServers_Pagination(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping db test in short mode")
	}

	userId := uuid.New()

	var recentServerIds []uuid.UUID
	numServersToCreate := 6
	lastSeenServerIdx := 0

	for i := 0; i < numServersToCreate; i++ {
		server := createTestServer(t, userId)
		require.NotEmpty(t, server)
		recentServerIds = append(recentServerIds, server.ID)
	}

	firstPageParams := ListUserServersParams{
		UserID:   userId,
		PageSize: 2,
	}
	firstPageUserServers, err := testStore.ListUserServers(context.Background(), firstPageParams)
	require.NoError(t, err)
	require.NotEmpty(t, firstPageUserServers)
	require.Len(t, firstPageUserServers, 2)

	for _, server := range firstPageUserServers {
		require.Equal(t, recentServerIds[len(recentServerIds)-lastSeenServerIdx-1], server.ID)
		lastSeenServerIdx += 1
	}

	lastPageParams := ListUserServersParams{
		UserID:   userId,
		PageSize: int32(numServersToCreate),
		AfterCreatedAt: pgtype.Timestamptz{
			Time:  firstPageUserServers[len(firstPageUserServers)-1].CreatedAt,
			Valid: true,
		},
		AfterID: pgtype.UUID{
			Bytes: firstPageUserServers[len(firstPageUserServers)-1].ID,
			Valid: true,
		},
	}
	lastPageUserServers, err := testStore.ListUserServers(context.Background(), lastPageParams)
	expectedNumLastPageServers := numServersToCreate - lastSeenServerIdx
	require.NoError(t, err)
	require.NotEmpty(t, lastPageUserServers)
	require.Len(t, lastPageUserServers, expectedNumLastPageServers)
	for _, server := range lastPageUserServers {
		require.Equal(t, recentServerIds[len(recentServerIds)-lastSeenServerIdx-1], server.ID)
		lastSeenServerIdx += 1
	}
}

func TestListUserServers_Permissions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping db test in short mode")
	}

	userId := uuid.New()

	// user owns a server with 2 other admins
	server1 := createTestServer(t, userId)
	require.NotEmpty(t, server1)
	admin1Server1 := createTestServerAdmin(t, server1.ID, uuid.New())
	require.NotEmpty(t, admin1Server1)
	admin2Server1 := createTestServerAdmin(t, server1.ID, uuid.New())
	require.NotEmpty(t, admin2Server1)

	// user is the admin of a server they don't own
	server2 := createTestServer(t, uuid.New())
	require.NotEmpty(t, server2)
	admin1Server2 := createTestServerAdmin(t, server2.ID, userId)
	require.NotEmpty(t, admin1Server2)

	// user is both the owner and an admin
	server3 := createTestServer(t, userId)
	require.NotEmpty(t, server3)
	admin1Server3 := createTestServerAdmin(t, server3.ID, userId)
	require.NotEmpty(t, admin1Server3)
	admin2Server3 := createTestServerAdmin(t, server3.ID, uuid.New())
	require.NotEmpty(t, admin2Server3)

	params := ListUserServersParams{
		UserID:   userId,
		PageSize: 5,
	}
	res, err := testStore.ListUserServers(context.Background(), params)
	require.NoError(t, err)
	require.NotEmpty(t, res)
	require.Len(t, res, 3)

	require.Equal(t, server3.ID, res[0].ID)
	require.Equal(t, server2.ID, res[1].ID)
	require.Equal(t, server1.ID, res[2].ID)
}

func TestGetTotalServerCount(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping db test in short mode")
	}

	initialServerCount, err := testStore.GetTotalServerCount(context.Background())
	require.NoError(t, err)
	require.NotNil(t, initialServerCount)

	numAdditionalServers := 5
	for i := 0; i < numAdditionalServers; i++ {
		server := createTestServer(t, uuid.New())
		require.NotEmpty(t, server)
	}

	finalServerCount, err := testStore.GetTotalServerCount(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, finalServerCount)

	expectedServerCount := initialServerCount + int64(numAdditionalServers)
	require.Equal(t, expectedServerCount, finalServerCount)
}

func TestListServers(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping db test in short mode")
	}

	var recentServerIds []uuid.UUID
	numServersToCreate := 6
	lastSeenServerIdx := 0

	for i := 0; i < numServersToCreate; i++ {
		server := createTestServer(t, uuid.New())
		require.NotEmpty(t, server)
		recentServerIds = append(recentServerIds, server.ID)
	}

	firstPageParams := ListServersParams{
		PageSize: 3,
	}
	firstPageServers, err := testStore.ListServers(context.Background(), firstPageParams)
	require.NoError(t, err)
	require.NotEmpty(t, firstPageServers)
	require.Len(t, firstPageServers, 3)

	for _, server := range firstPageServers {
		require.Equal(t, recentServerIds[len(recentServerIds)-lastSeenServerIdx-1], server.ID)
		lastSeenServerIdx += 1
	}

	lastPageParams := ListServersParams{
		PageSize: int32(numServersToCreate),
		AfterCreatedAt: pgtype.Timestamptz{
			Time:  firstPageServers[len(firstPageServers)-1].CreatedAt,
			Valid: true,
		},
		AfterID: pgtype.UUID{
			Bytes: firstPageServers[len(firstPageServers)-1].ID,
			Valid: true,
		},
	}
	lastPageServers, err := testStore.ListServers(context.Background(), lastPageParams)
	expectedNumLastPageServers := numServersToCreate - lastSeenServerIdx
	require.NoError(t, err)
	require.NotEmpty(t, lastPageServers)
	require.GreaterOrEqual(t, len(lastPageServers), expectedNumLastPageServers)
	for i := 0; i < expectedNumLastPageServers; i++ {
		server := lastPageServers[i]
		require.Equal(t, recentServerIds[len(recentServerIds)-lastSeenServerIdx-1], server.ID)
		lastSeenServerIdx += 1
	}
}

func TestUpdateServer(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping db test in short mode")
	}

	server := createTestServer(t, uuid.New())
	require.NotEmpty(t, server)

	updatedServerWallet, err := commonUtil.NewEthereumWallet()
	require.NoError(t, err)
	require.NotEmpty(t, updatedServerWallet)

	expectedUpdatedStakePerGameStr := "80000000000000000"
	expectedUpdatedStakePerGame, err := ParseWeiStrToBigInt(expectedUpdatedStakePerGameStr)
	require.NoError(t, err)
	require.NotEmpty(t, expectedUpdatedStakePerGame)

	expectedUpdatedName := fmt.Sprintf("%s--updated", server.Name)
	expectedUpdatedOwnerId := uuid.New()
	expectedUpdatedNumAdmins := 2
	expectedUpdatedNumCustomWords := 3
	expectedUpdatedServerAddress := updatedServerWallet.Address
	expectedUpdatedNumRoundsPerGame := 3
	expectedUpdatedRoundDurationSecs := 10 * 60
	expectedUpdatedNumDrawingOptions := 5
	expectedUpdatedArchivedAt := time.Now().UTC()

	params := UpdateServerParams{
		ServerID: server.ID,
		Name: pgtype.Text{
			String: expectedUpdatedName,
			Valid:  true,
		},
		OwnerID: pgtype.UUID{
			Bytes: expectedUpdatedOwnerId,
			Valid: true,
		},
		NumAdmins: pgtype.Int4{
			Int32: int32(expectedUpdatedNumAdmins),
			Valid: true,
		},
		NumCustomWords: pgtype.Int4{
			Int32: int32(expectedUpdatedNumCustomWords),
			Valid: true,
		},
		ServerAddress: pgtype.Text{
			String: expectedUpdatedServerAddress,
			Valid:  true,
		},
		StakePerGame: pgtype.Numeric{
			Int:   expectedUpdatedStakePerGame,
			Valid: true,
		},
		NumRoundsPerGame: pgtype.Int4{
			Int32: int32(expectedUpdatedNumRoundsPerGame),
			Valid: true,
		},
		RoundDurationSecs: pgtype.Int4{
			Int32: int32(expectedUpdatedRoundDurationSecs),
			Valid: true,
		},
		NumDrawingOptions: pgtype.Int4{
			Int32: int32(expectedUpdatedNumDrawingOptions),
			Valid: true,
		},
		IsArchived: pgtype.Bool{
			Bool:  true,
			Valid: true,
		},
		ArchivedAt: pgtype.Timestamptz{
			Time:  expectedUpdatedArchivedAt,
			Valid: true,
		},
	}

	updatedServer, err := testStore.UpdateServer(context.Background(), params)
	require.NoError(t, err)
	require.NotEmpty(t, updatedServer)

	require.Equal(t, server.ID, updatedServer.ID)
	require.WithinDuration(t, server.CreatedAt, updatedServer.CreatedAt, time.Second)

	updatedStakePerGameStr, err := ParseDBNumericWeiToStr(updatedServer.StakePerGame)
	require.NoError(t, err)
	require.NotEmpty(t, updatedStakePerGameStr)
	require.Equal(t, expectedUpdatedStakePerGameStr, updatedStakePerGameStr)

	require.Equal(t, expectedUpdatedName, updatedServer.Name)
	require.Equal(t, expectedUpdatedOwnerId, updatedServer.OwnerID)
	require.Equal(t, int32(expectedUpdatedNumAdmins), updatedServer.NumAdmins)
	require.Equal(t, int32(expectedUpdatedNumCustomWords), updatedServer.NumCustomWords)
	require.Equal(t, expectedUpdatedServerAddress, updatedServer.ServerAddress)
	require.Equal(t, int32(expectedUpdatedNumRoundsPerGame), updatedServer.NumRoundsPerGame)
	require.Equal(t, int32(expectedUpdatedRoundDurationSecs), updatedServer.RoundDurationSecs)
	require.Equal(t, int32(expectedUpdatedNumDrawingOptions), updatedServer.NumDrawingOptions)
	require.True(t, updatedServer.IsArchived)
	require.WithinDuration(t, expectedUpdatedArchivedAt, updatedServer.ArchivedAt.Time, time.Second)
}

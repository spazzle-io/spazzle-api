package db

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

func createTestServerAdmin(t *testing.T, serverId uuid.UUID, userId uuid.UUID) ServerAdmin {
	params := AddServerAdminParams{
		ServerID: serverId,
		UserID:   userId,
	}

	serverAdmin, err := testStore.AddServerAdmin(context.Background(), params)
	require.NoError(t, err)
	require.NotEmpty(t, serverAdmin)

	require.Equal(t, serverId, serverAdmin.ServerID)
	require.Equal(t, userId, serverAdmin.UserID)
	require.WithinDuration(t, time.Now().UTC(), serverAdmin.AddedAt, time.Second)

	return serverAdmin
}

func TestAddServerAdmin(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping db test in short mode")
	}

	userId := uuid.New()

	server := createTestServer(t, userId)
	require.NotEmpty(t, server)

	serverAdmin := createTestServerAdmin(t, server.ID, userId)
	require.NotEmpty(t, serverAdmin)
}

func TestListServerAdmins(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping db test in short mode")
	}

	server := createTestServer(t, uuid.New())
	require.NotEmpty(t, server)

	var recentlyAddedServerAdmins []uuid.UUID
	numServerAdminsToCreate := 5
	lastSeenServerAdminIdx := 0

	for i := 0; i < numServerAdminsToCreate; i++ {
		serverAdmin := createTestServerAdmin(t, server.ID, uuid.New())
		require.NotEmpty(t, serverAdmin)
		recentlyAddedServerAdmins = append(recentlyAddedServerAdmins, serverAdmin.UserID)
	}

	firstPageParams := ListServerAdminsParams{
		ServerID: server.ID,
		PageSize: 2,
	}
	firstPageAdmins, err := testStore.ListServerAdmins(context.Background(), firstPageParams)
	require.NoError(t, err)
	require.NotEmpty(t, firstPageAdmins)
	require.Len(t, firstPageAdmins, 2)

	for _, serverAdmin := range firstPageAdmins {
		require.Equal(t, recentlyAddedServerAdmins[len(recentlyAddedServerAdmins)-lastSeenServerAdminIdx-1], serverAdmin.UserID)
		lastSeenServerAdminIdx += 1
	}

	lastPageParams := ListServerAdminsParams{
		ServerID: server.ID,
		PageSize: int32(numServerAdminsToCreate),
		AfterAddedAt: pgtype.Timestamptz{
			Time:  firstPageAdmins[len(firstPageAdmins)-1].AddedAt,
			Valid: true,
		},
		AfterUserID: pgtype.UUID{
			Bytes: firstPageAdmins[len(firstPageAdmins)-1].UserID,
			Valid: true,
		},
	}
	lastPageAdmins, err := testStore.ListServerAdmins(context.Background(), lastPageParams)
	expectedNumLastPageAdmins := numServerAdminsToCreate - lastSeenServerAdminIdx
	require.NoError(t, err)
	require.NotEmpty(t, lastPageAdmins)
	require.Len(t, lastPageAdmins, expectedNumLastPageAdmins)

	for _, serverAdmin := range lastPageAdmins {
		require.Equal(t, recentlyAddedServerAdmins[len(recentlyAddedServerAdmins)-lastSeenServerAdminIdx-1], serverAdmin.UserID)
		lastSeenServerAdminIdx += 1
	}
}

func TestRemoveServerAdmin(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping db test in short mode")
	}

	userId := uuid.New()

	server := createTestServer(t, userId)
	require.NotEmpty(t, server)

	serverAdmin := createTestServerAdmin(t, server.ID, userId)
	require.NotEmpty(t, serverAdmin)

	serverAdmins, err := testStore.ListServerAdmins(context.Background(), ListServerAdminsParams{
		ServerID: server.ID,
		PageSize: 1,
	})
	require.NoError(t, err)
	require.Len(t, serverAdmins, 1)
	require.Equal(t, serverAdmins[0].UserID, userId)

	params := RemoveServerAdminParams{
		ServerID: server.ID,
		UserID:   userId,
	}
	ct, err := testStore.RemoveServerAdmin(context.Background(), params)
	require.NoError(t, err)
	require.Equal(t, int64(1), ct.RowsAffected())

	params = RemoveServerAdminParams{
		ServerID: server.ID,
		UserID:   userId,
	}
	ct, err = testStore.RemoveServerAdmin(context.Background(), params)
	require.NoError(t, err)
	require.Equal(t, int64(0), ct.RowsAffected())

	serverAdmins, err = testStore.ListServerAdmins(context.Background(), ListServerAdminsParams{
		ServerID: server.ID,
		PageSize: 1,
	})
	require.NoError(t, err)
	require.Len(t, serverAdmins, 0)
}

package gameserver

import (
	"testing"

	mockdb "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/mock"
	db "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/sqlc"
	mockwordstore "github.com/spazzle-io/spazzle-api/services/gameplay/internal/wordstore/mock"

	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/eventbus"
	mockeventbus "github.com/spazzle-io/spazzle-api/services/gameplay/internal/eventbus/mock"
	mockgameflowclient "github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow/mock"
	"go.uber.org/mock/gomock"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func createTestGameServerManager(t *testing.T) *Manager {
	gameServerManager := NewManager()
	require.NotEmpty(t, gameServerManager)
	require.Empty(t, gameServerManager.gameServers)

	return gameServerManager
}

func TestCreateGameServerManager(t *testing.T) {
	gameServerManager := createTestGameServerManager(t)
	require.NotEmpty(t, gameServerManager)
}

func TestGetOrCreateGameServer(t *testing.T) {
	crtl := gomock.NewController(t)
	defer crtl.Finish()

	store := mockdb.NewMockStore(crtl)
	bus := mockeventbus.NewMockEventBus(crtl)
	session := mockeventbus.NewMockSession(crtl)
	gfClient := mockgameflowclient.NewMockClient(crtl)
	wordStore := mockwordstore.NewMockStore(crtl)

	sm := createTestGameServerManager(t)
	require.NotEmpty(t, sm)

	serverID := uuid.New()
	gameID := uuid.New()

	store.EXPECT().
		GetServerById(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.Server{
			NumDrawingOptions: 3,
		}, nil)

	gfClient.EXPECT().
		Game(gomock.Eq(serverID), gomock.Any()).
		Times(1).
		Return(gameID, nil)

	bus.EXPECT().
		Session(gomock.Eq(eventbus.GameIdentifier{
			GameID:       gameID,
			GameServerID: serverID,
		})).
		Times(1).
		Return(session, nil)

	session.EXPECT().
		Subscribe(gomock.Any(), gomock.Eq(eventbus.GameEventsStreamType), gomock.Eq(eventbus.StartFromNow()), gomock.Any()).
		Times(1).
		Return(nil)

	session.EXPECT().
		Subscribe(gomock.Any(), gomock.Eq(eventbus.DrawingUpdatesStreamType), gomock.Eq(eventbus.StartFromNow()), gomock.Any()).
		Times(1).
		Return(nil)

	gfClient.EXPECT().
		HeartbeatGameServerInstance(gomock.Eq(serverID), gomock.Any()).
		AnyTimes().
		Return(nil)

	config := &Config{
		Store:     store,
		Bus:       bus,
		GfClient:  gfClient,
		WordStore: wordStore,
	}

	gsCall1, err := sm.GetOrCreateGameServer(serverID, config)
	require.NoError(t, err)
	require.NotEmpty(t, gsCall1)

	gsCall2, err := sm.GetOrCreateGameServer(serverID, config)
	require.NoError(t, err)
	require.NotEmpty(t, gsCall2)

	require.Equal(t, gsCall1, gsCall2)
}

func TestRemoveGameServerIfClosed(t *testing.T) {
	crtl := gomock.NewController(t)
	defer crtl.Finish()

	store := mockdb.NewMockStore(crtl)
	bus := mockeventbus.NewMockEventBus(crtl)
	session := mockeventbus.NewMockSession(crtl)
	gfClient := mockgameflowclient.NewMockClient(crtl)
	wordStore := mockwordstore.NewMockStore(crtl)

	sm := createTestGameServerManager(t)
	require.NotEmpty(t, sm)

	serverID := uuid.New()
	gameID := uuid.New()

	store.EXPECT().
		GetServerById(gomock.Any(), gomock.Eq(serverID)).
		Times(1).
		Return(db.Server{
			NumDrawingOptions: 3,
		}, nil)

	gfClient.EXPECT().
		Game(gomock.Eq(serverID), gomock.Any()).
		Times(1).
		Return(gameID, nil)

	bus.EXPECT().
		Session(gomock.Eq(eventbus.GameIdentifier{
			GameID:       gameID,
			GameServerID: serverID,
		})).
		Times(1).
		Return(session, nil)

	session.EXPECT().
		Subscribe(gomock.Any(), gomock.Eq(eventbus.GameEventsStreamType), gomock.Eq(eventbus.StartFromNow()), gomock.Any()).
		Times(1).
		Return(nil)

	session.EXPECT().
		Subscribe(gomock.Any(), gomock.Eq(eventbus.DrawingUpdatesStreamType), gomock.Eq(eventbus.StartFromNow()), gomock.Any()).
		Times(1).
		Return(nil)

	gfClient.EXPECT().
		HeartbeatGameServerInstance(gomock.Eq(serverID), gomock.Any()).
		AnyTimes().
		Return(nil)

	session.EXPECT().
		Close().
		Times(1).
		Return()

	gfClient.EXPECT().
		UnregisterGameServerInstance(gomock.Eq(serverID), gomock.Any()).
		Times(1).
		Return(nil)

	gfClient.EXPECT().
		Flush().
		Times(1).
		Return()

	config := &Config{
		Store:     store,
		Bus:       bus,
		GfClient:  gfClient,
		WordStore: wordStore,
	}

	gs, err := sm.GetOrCreateGameServer(serverID, config)
	require.NoError(t, err)
	gs.shutdown()

	require.NotEmpty(t, sm.gameServers[serverID])

	sm.RemoveGameServerIfClosed(serverID)

	require.Nil(t, sm.gameServers[serverID])
}

func TestManagerShutdown(t *testing.T) {
	crtl := gomock.NewController(t)
	defer crtl.Finish()

	store := mockdb.NewMockStore(crtl)
	bus := mockeventbus.NewMockEventBus(crtl)
	session := mockeventbus.NewMockSession(crtl)
	gfClient := mockgameflowclient.NewMockClient(crtl)
	wordStore := mockwordstore.NewMockStore(crtl)

	sm := createTestGameServerManager(t)
	require.NotEmpty(t, sm)

	serverID := uuid.New()
	gameID := uuid.New()

	store.EXPECT().
		GetServerById(gomock.Any(), gomock.Eq(serverID)).
		Times(1).
		Return(db.Server{
			NumDrawingOptions: 3,
		}, nil)

	gfClient.EXPECT().
		Game(gomock.Eq(serverID), gomock.Any()).
		Times(1).
		Return(gameID, nil)

	bus.EXPECT().
		Session(gomock.Eq(eventbus.GameIdentifier{
			GameID:       gameID,
			GameServerID: serverID,
		})).
		Times(1).
		Return(session, nil)

	session.EXPECT().
		Subscribe(gomock.Any(), gomock.Eq(eventbus.GameEventsStreamType), gomock.Eq(eventbus.StartFromNow()), gomock.Any()).
		Times(1).
		Return(nil)

	session.EXPECT().
		Subscribe(gomock.Any(), gomock.Eq(eventbus.DrawingUpdatesStreamType), gomock.Eq(eventbus.StartFromNow()), gomock.Any()).
		Times(1).
		Return(nil)

	gfClient.EXPECT().
		HeartbeatGameServerInstance(gomock.Eq(serverID), gomock.Any()).
		AnyTimes().
		Return(nil)

	session.EXPECT().
		Close().
		Times(1).
		Return()

	gfClient.EXPECT().
		UnregisterGameServerInstance(gomock.Eq(serverID), gomock.Any()).
		Times(1).
		Return(nil)

	gfClient.EXPECT().
		Flush().
		Times(1).
		Return()

	config := &Config{
		Store:     store,
		Bus:       bus,
		GfClient:  gfClient,
		WordStore: wordStore,
	}

	gs, err := sm.GetOrCreateGameServer(serverID, config)
	require.NoError(t, err)
	gs.shutdown()

	require.NotEmpty(t, sm.gameServers[serverID])

	sm.Shutdown()

	require.Nil(t, sm.gameServers[serverID])
}

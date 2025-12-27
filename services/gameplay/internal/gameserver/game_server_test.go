package gameserver

import (
	"testing"
	"time"

	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/eventbus"
	mockeventbus "github.com/spazzle-io/spazzle-api/services/gameplay/internal/eventbus/mock"
	mockgameflowclient "github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow/mock"

	"go.uber.org/mock/gomock"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func createTestGameServer(t *testing.T) (
	*mockeventbus.MockEventBus,
	*mockeventbus.MockSession,
	*mockgameflowclient.MockClient,
	*GameServer,
) {
	serverID := uuid.New()
	gameID := uuid.New()

	crtl := gomock.NewController(t)
	defer crtl.Finish()

	bus := mockeventbus.NewMockEventBus(crtl)
	session := mockeventbus.NewMockSession(crtl)
	gfClient := mockgameflowclient.NewMockClient(crtl)

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

	gameServer, err := NewGameServer(serverID, bus, gfClient, &NewGameServerOptions{StartServer: false})
	require.NoError(t, err)
	require.NotEmpty(t, gameServer)
	require.Equal(t, serverID, gameServer.serverID)

	return bus, session, gfClient, gameServer
}

func TestCreateGameServer(t *testing.T) {
	mockEventBus, mockBusSession, mockGfClient, gameServer := createTestGameServer(t)
	require.NotEmpty(t, mockEventBus)
	require.NotEmpty(t, mockBusSession)
	require.NotEmpty(t, mockGfClient)
	require.NotEmpty(t, gameServer)
}

func TestAddClient(t *testing.T) {
	_, _, mockGfClient, gameServer := createTestGameServer(t)
	require.NotEmpty(t, mockGfClient)
	require.NotEmpty(t, gameServer)

	client := &Client{
		userID:     uuid.New(),
		connID:     uuid.New(),
		gameServer: gameServer,
		send:       make(chan OutgoingMessage, ClientSendChanBufSize),
	}

	require.Len(t, gameServer.clients, 0)
	require.Len(t, gameServer.clients[client.userID], 0)
	require.Equal(t, gameServer.clientCount.Load(), int32(0))
	require.Equal(t, gameServer.connCount.Load(), int32(0))

	mockGfClient.EXPECT().
		AddPlayers(gomock.Eq(gameServer.serverID), gomock.Eq([]uuid.UUID{client.userID})).
		Times(1).
		Return()

	gameServer.addClient(client)

	require.Len(t, gameServer.clients, 1)
	require.Len(t, gameServer.clients[client.userID], 1)
	require.Equal(t, gameServer.clientCount.Load(), int32(1))
	require.Equal(t, gameServer.connCount.Load(), int32(1))
}

func TestAddClient_IsSpectating(t *testing.T) {
	_, _, mockGfClient, gameServer := createTestGameServer(t)
	require.NotEmpty(t, mockGfClient)
	require.NotEmpty(t, gameServer)

	client := &Client{
		userID:       uuid.New(),
		connID:       uuid.New(),
		isSpectating: true,
		gameServer:   gameServer,
		send:         make(chan OutgoingMessage, ClientSendChanBufSize),
	}

	require.Len(t, gameServer.clients, 0)
	require.Len(t, gameServer.clients[client.userID], 0)
	require.Equal(t, gameServer.clientCount.Load(), int32(0))
	require.Equal(t, gameServer.connCount.Load(), int32(0))

	gameServer.addClient(client)

	require.Len(t, gameServer.clients, 1)
	require.Len(t, gameServer.clients[client.userID], 1)
	require.Equal(t, gameServer.clientCount.Load(), int32(1))
	require.Equal(t, gameServer.connCount.Load(), int32(1))
}

func TestGetClientConnections(t *testing.T) {
	_, _, mockGfClient, gameServer := createTestGameServer(t)
	require.NotEmpty(t, mockGfClient)
	require.NotEmpty(t, gameServer)

	userID := uuid.New()

	mockGfClient.EXPECT().
		AddPlayers(gomock.Any(), gomock.Any()).
		Times(2).
		Return()

	clientConns := gameServer.GetClientConnections(userID)
	require.Empty(t, clientConns)
	require.Len(t, clientConns, 0)

	client1 := &Client{
		userID:     userID,
		connID:     uuid.New(),
		gameServer: gameServer,
		send:       make(chan OutgoingMessage, ClientSendChanBufSize),
	}
	gameServer.addClient(client1)

	clientConns = gameServer.GetClientConnections(userID)
	require.NotEmpty(t, clientConns)
	require.Len(t, clientConns, 1)

	client2 := &Client{
		userID:     userID,
		connID:     uuid.New(),
		gameServer: gameServer,
		send:       make(chan OutgoingMessage, ClientSendChanBufSize),
	}
	gameServer.addClient(client2)

	clientConns = gameServer.GetClientConnections(userID)
	require.NotEmpty(t, clientConns)
	require.Len(t, clientConns, 2)
}

func TestRemoveClient(t *testing.T) {
	_, _, mockGfClient, gameServer := createTestGameServer(t)
	require.NotEmpty(t, mockGfClient)
	require.NotEmpty(t, gameServer)

	client := &Client{
		userID:     uuid.New(),
		connID:     uuid.New(),
		gameServer: gameServer,
		send:       make(chan OutgoingMessage, ClientSendChanBufSize),
	}

	mockGfClient.EXPECT().
		AddPlayers(gomock.Eq(gameServer.serverID), gomock.Eq([]uuid.UUID{client.userID})).
		Times(1).
		Return()

	gameServer.addClient(client)

	require.Len(t, gameServer.clients, 1)
	require.Len(t, gameServer.clients[client.userID], 1)
	require.Equal(t, gameServer.clientCount.Load(), int32(1))
	require.Equal(t, gameServer.connCount.Load(), int32(1))

	mockGfClient.EXPECT().
		RemovePlayers(gomock.Eq(gameServer.serverID), gomock.Eq([]uuid.UUID{client.userID})).
		Times(1).
		Return()

	gameServer.removeClient(client)

	require.Len(t, gameServer.clients, 0)
	require.Len(t, gameServer.clients[client.userID], 0)
	require.Equal(t, gameServer.clientCount.Load(), int32(0))
	require.Equal(t, gameServer.connCount.Load(), int32(0))
}

func TestRemoveClient_IsSpectating(t *testing.T) {
	_, _, mockGfClient, gameServer := createTestGameServer(t)
	require.NotEmpty(t, mockGfClient)
	require.NotEmpty(t, gameServer)

	client := &Client{
		userID:       uuid.New(),
		connID:       uuid.New(),
		isSpectating: true,
		gameServer:   gameServer,
		send:         make(chan OutgoingMessage, ClientSendChanBufSize),
	}

	gameServer.addClient(client)

	require.Len(t, gameServer.clients, 1)
	require.Len(t, gameServer.clients[client.userID], 1)
	require.Equal(t, gameServer.clientCount.Load(), int32(1))
	require.Equal(t, gameServer.connCount.Load(), int32(1))

	gameServer.removeClient(client)

	require.Len(t, gameServer.clients, 0)
	require.Len(t, gameServer.clients[client.userID], 0)
	require.Equal(t, gameServer.clientCount.Load(), int32(0))
	require.Equal(t, gameServer.connCount.Load(), int32(0))
}

func TestRemoveClient_UserIdNotRegistered(t *testing.T) {
	_, _, mockGfClient, gameServer := createTestGameServer(t)
	require.NotEmpty(t, mockGfClient)
	require.NotEmpty(t, gameServer)

	client := &Client{
		userID:     uuid.New(),
		connID:     uuid.New(),
		gameServer: gameServer,
		send:       make(chan OutgoingMessage, ClientSendChanBufSize),
	}

	mockGfClient.EXPECT().
		AddPlayers(gomock.Eq(gameServer.serverID), gomock.Eq([]uuid.UUID{client.userID})).
		Times(1).
		Return()

	gameServer.addClient(client)

	originalClientUserID := client.userID
	client.userID = uuid.New()
	gameServer.removeClient(client)

	require.Len(t, gameServer.clients, 1)
	require.Len(t, gameServer.clients[originalClientUserID], 1)
	require.Equal(t, gameServer.clientCount.Load(), int32(1))
	require.Equal(t, gameServer.connCount.Load(), int32(1))
}

func TestRemoveClient_UserConnNotRegistered(t *testing.T) {
	_, _, mockGfClient, gameServer := createTestGameServer(t)
	require.NotEmpty(t, mockGfClient)
	require.NotEmpty(t, gameServer)

	client := &Client{
		userID:     uuid.New(),
		connID:     uuid.New(),
		gameServer: gameServer,
		send:       make(chan OutgoingMessage, ClientSendChanBufSize),
	}

	mockGfClient.EXPECT().
		AddPlayers(gomock.Eq(gameServer.serverID), gomock.Eq([]uuid.UUID{client.userID})).
		Times(1).
		Return()

	gameServer.addClient(client)

	client.connID = uuid.New()
	gameServer.removeClient(client)

	require.Len(t, gameServer.clients, 1)
	require.Len(t, gameServer.clients[client.userID], 1)
	require.Equal(t, gameServer.clientCount.Load(), int32(1))
	require.Equal(t, gameServer.connCount.Load(), int32(1))
}

func TestRemoveAllClients(t *testing.T) {
	_, _, mockGfClient, gameServer := createTestGameServer(t)
	require.NotEmpty(t, mockGfClient)
	require.NotEmpty(t, gameServer)

	client1 := &Client{
		userID:     uuid.New(),
		connID:     uuid.New(),
		gameServer: gameServer,
		send:       make(chan OutgoingMessage, ClientSendChanBufSize),
	}

	client2 := &Client{
		userID:       uuid.New(),
		connID:       uuid.New(),
		isSpectating: true,
		gameServer:   gameServer,
		send:         make(chan OutgoingMessage, ClientSendChanBufSize),
	}

	mockGfClient.EXPECT().
		AddPlayers(gomock.Eq(gameServer.serverID), gomock.Eq([]uuid.UUID{client1.userID})).
		Times(2).
		Return()

	gameServer.addClient(client1)
	gameServer.addClient(client2)

	require.Len(t, gameServer.clients, 2)
	require.Len(t, gameServer.clients[client1.userID], 1)
	require.Len(t, gameServer.clients[client2.userID], 1)
	require.Equal(t, gameServer.clientCount.Load(), int32(2))
	require.Equal(t, gameServer.connCount.Load(), int32(2))

	mockGfClient.EXPECT().
		RemovePlayers(gomock.Eq(gameServer.serverID), gomock.Eq([]uuid.UUID{client1.userID})).
		Times(1).
		Return()

	gameServer.removeAllClients()

	require.Len(t, gameServer.clients, 0)
	require.Len(t, gameServer.clients[client1.userID], 0)
	require.Len(t, gameServer.clients[client2.userID], 0)
	require.Equal(t, gameServer.clientCount.Load(), int32(0))
	require.Equal(t, gameServer.connCount.Load(), int32(0))
}

func TestDispatchMsg(t *testing.T) {
	_, _, mockGfClient, gameServer := createTestGameServer(t)
	require.NotEmpty(t, mockGfClient)
	require.NotEmpty(t, gameServer)

	client := &Client{
		userID:     uuid.New(),
		connID:     uuid.New(),
		gameServer: gameServer,
		send:       make(chan OutgoingMessage, ClientSendChanBufSize),
	}

	mockGfClient.EXPECT().
		AddPlayers(gomock.Eq(gameServer.serverID), gomock.Eq([]uuid.UUID{client.userID})).
		Times(1).
		Return()

	gameServer.addClient(client)

	msg := []byte("test msg")
	gameServer.dispatchMsg(msg)

	retrievedMsg := <-client.send
	require.Equal(t, msg, retrievedMsg.Data)
	require.False(t, retrievedMsg.RequiresWorkflowAck)
	require.Empty(t, retrievedMsg.CorrelationID)
}

func TestDispatchDirectMsg_RecipientUserIdNotFound(t *testing.T) {
	_, _, mockGfClient, gameServer := createTestGameServer(t)
	require.NotEmpty(t, mockGfClient)
	require.NotEmpty(t, gameServer)

	client := &Client{
		userID:     uuid.New(),
		connID:     uuid.New(),
		gameServer: gameServer,
		send:       make(chan OutgoingMessage, ClientSendChanBufSize),
	}

	mockGfClient.EXPECT().
		AddPlayers(gomock.Eq(gameServer.serverID), gomock.Eq([]uuid.UUID{client.userID})).
		Times(1).
		Return()

	gameServer.addClient(client)

	testMsg := []byte("test msg")
	payload := &DirectMsgPayload{
		Recipients: []DirectMsgRecipient{
			{
				UserID:  uuid.New(),
				ConnIDs: []uuid.UUID{client.connID},
			},
		},
		Msg: OutgoingMessage{
			Data: testMsg,
		},
	}

	gameServer.dispatchDirectMsg(payload)

	require.Len(t, client.send, 0)
}

func TestDispatchDirectMsg_RecipientConnIdNotFound(t *testing.T) {
	_, _, mockGfClient, gameServer := createTestGameServer(t)
	require.NotEmpty(t, mockGfClient)
	require.NotEmpty(t, gameServer)

	client := &Client{
		userID:     uuid.New(),
		connID:     uuid.New(),
		gameServer: gameServer,
		send:       make(chan OutgoingMessage, ClientSendChanBufSize),
	}

	mockGfClient.EXPECT().
		AddPlayers(gomock.Eq(gameServer.serverID), gomock.Eq([]uuid.UUID{client.userID})).
		Times(1).
		Return()

	gameServer.addClient(client)

	testMsg := []byte("test msg")
	payload := &DirectMsgPayload{
		Recipients: []DirectMsgRecipient{
			{
				UserID:  client.userID,
				ConnIDs: []uuid.UUID{uuid.New()},
			},
		},
		Msg: OutgoingMessage{
			Data: testMsg,
		},
	}

	gameServer.dispatchDirectMsg(payload)

	require.Len(t, client.send, 0)
}

func TestDispatchDirectMsg_AllUserConnections(t *testing.T) {
	_, _, mockGfClient, gameServer := createTestGameServer(t)
	require.NotEmpty(t, mockGfClient)
	require.NotEmpty(t, gameServer)

	userId := uuid.New()

	client1 := &Client{
		userID:       userId,
		connID:       uuid.New(),
		gameServer:   gameServer,
		isSpectating: true,
		send:         make(chan OutgoingMessage, ClientSendChanBufSize),
	}

	client2 := &Client{
		userID:       userId,
		connID:       uuid.New(),
		gameServer:   gameServer,
		isSpectating: false,
		send:         make(chan OutgoingMessage, ClientSendChanBufSize),
	}

	mockGfClient.EXPECT().
		AddPlayers(gomock.Eq(gameServer.serverID), gomock.AnyOf([]uuid.UUID{client1.userID}, []uuid.UUID{client2.userID})).
		Times(2).
		Return()

	gameServer.addClient(client1)
	gameServer.addClient(client2)

	testMsg := []byte("test msg")
	payload := &DirectMsgPayload{
		Recipients: []DirectMsgRecipient{
			{
				UserID: userId,
			},
		},
		Msg: OutgoingMessage{
			Data: testMsg,
		},
	}

	gameServer.dispatchDirectMsg(payload)

	retrievedMsgClient1 := <-client1.send
	retrievedMsgClient2 := <-client2.send
	require.Equal(t, testMsg, retrievedMsgClient1.Data)
	require.Equal(t, testMsg, retrievedMsgClient2.Data)
}

func TestDispatchDirectMsg_SpecificUserConnections(t *testing.T) {
	_, _, mockGfClient, gameServer := createTestGameServer(t)
	require.NotEmpty(t, mockGfClient)
	require.NotEmpty(t, gameServer)

	userId := uuid.New()

	client1 := &Client{
		userID:     userId,
		connID:     uuid.New(),
		gameServer: gameServer,
		send:       make(chan OutgoingMessage, ClientSendChanBufSize),
	}

	client2 := &Client{
		userID:       userId,
		connID:       uuid.New(),
		gameServer:   gameServer,
		isSpectating: true,
		send:         make(chan OutgoingMessage, ClientSendChanBufSize),
	}

	client3 := &Client{
		userID:     userId,
		connID:     uuid.New(),
		gameServer: gameServer,
		send:       make(chan OutgoingMessage, ClientSendChanBufSize),
	}

	mockGfClient.EXPECT().
		AddPlayers(gomock.Eq(gameServer.serverID), gomock.AnyOf([]uuid.UUID{client1.userID}, []uuid.UUID{client2.userID}, []uuid.UUID{client3.userID})).
		Times(3).
		Return()

	gameServer.addClient(client1)
	gameServer.addClient(client2)
	gameServer.addClient(client3)

	testMsg := []byte("test msg")
	payload := &DirectMsgPayload{
		Recipients: []DirectMsgRecipient{
			{
				UserID:  userId,
				ConnIDs: []uuid.UUID{client2.connID, client3.connID},
			},
		},
		Msg: OutgoingMessage{
			Data: testMsg,
		},
	}

	gameServer.dispatchDirectMsg(payload)

	retrievedMsgClient2 := <-client2.send
	retrievedMsgClient3 := <-client3.send
	require.Len(t, client1.send, 0)
	require.Equal(t, testMsg, retrievedMsgClient2.Data)
	require.Equal(t, testMsg, retrievedMsgClient3.Data)
}

func TestDispatchDirectMsg_SpecificUserConnections_ExcludeSpectators(t *testing.T) {
	_, _, mockGfClient, gameServer := createTestGameServer(t)
	require.NotEmpty(t, mockGfClient)
	require.NotEmpty(t, gameServer)

	userId := uuid.New()

	client1 := &Client{
		userID:     userId,
		connID:     uuid.New(),
		gameServer: gameServer,
		send:       make(chan OutgoingMessage, ClientSendChanBufSize),
	}

	client2 := &Client{
		userID:       userId,
		connID:       uuid.New(),
		gameServer:   gameServer,
		isSpectating: true,
		send:         make(chan OutgoingMessage, ClientSendChanBufSize),
	}

	client3 := &Client{
		userID:     userId,
		connID:     uuid.New(),
		gameServer: gameServer,
		send:       make(chan OutgoingMessage, ClientSendChanBufSize),
	}

	mockGfClient.EXPECT().
		AddPlayers(gomock.Eq(gameServer.serverID), gomock.AnyOf([]uuid.UUID{client1.userID}, []uuid.UUID{client2.userID}, []uuid.UUID{client3.userID})).
		Times(3).
		Return()

	gameServer.addClient(client1)
	gameServer.addClient(client2)
	gameServer.addClient(client3)

	testMsg := []byte("test msg")
	payload := &DirectMsgPayload{
		Recipients: []DirectMsgRecipient{
			{
				UserID:            userId,
				ConnIDs:           []uuid.UUID{client2.connID, client3.connID},
				ExcludeSpectators: true,
			},
		},
		Msg: OutgoingMessage{
			Data: testMsg,
		},
	}

	gameServer.dispatchDirectMsg(payload)

	retrievedMsgClient3 := <-client3.send
	require.Len(t, client1.send, 0)
	require.Len(t, client2.send, 0)
	require.Equal(t, testMsg, retrievedMsgClient3.Data)
}

func TestDispatchDirectMsg_ExcludeSpectators(t *testing.T) {
	_, _, mockGfClient, gameServer := createTestGameServer(t)
	require.NotEmpty(t, mockGfClient)
	require.NotEmpty(t, gameServer)

	userId := uuid.New()

	client1 := &Client{
		userID:     userId,
		connID:     uuid.New(),
		gameServer: gameServer,
		send:       make(chan OutgoingMessage, ClientSendChanBufSize),
	}

	client2 := &Client{
		userID:       userId,
		connID:       uuid.New(),
		gameServer:   gameServer,
		isSpectating: true,
		send:         make(chan OutgoingMessage, ClientSendChanBufSize),
	}

	client3 := &Client{
		userID:     userId,
		connID:     uuid.New(),
		gameServer: gameServer,
		send:       make(chan OutgoingMessage, ClientSendChanBufSize),
	}

	mockGfClient.EXPECT().
		AddPlayers(gomock.Eq(gameServer.serverID), gomock.AnyOf([]uuid.UUID{client1.userID}, []uuid.UUID{client2.userID}, []uuid.UUID{client3.userID})).
		Times(3).
		Return()

	gameServer.addClient(client1)
	gameServer.addClient(client2)
	gameServer.addClient(client3)

	testMsg := []byte("test msg")
	payload := &DirectMsgPayload{
		Recipients: []DirectMsgRecipient{
			{
				UserID:            userId,
				ExcludeSpectators: true,
			},
		},
		Msg: OutgoingMessage{
			Data: testMsg,
		},
	}

	gameServer.dispatchDirectMsg(payload)

	retrievedMsgClient1 := <-client1.send
	retrievedMsgClient3 := <-client3.send
	require.Len(t, client2.send, 0)
	require.Equal(t, testMsg, retrievedMsgClient1.Data)
	require.Equal(t, testMsg, retrievedMsgClient3.Data)
}

func TestScheduleShutdown(t *testing.T) {
	_, _, _, gameServer := createTestGameServer(t)
	require.NotEmpty(t, gameServer)

	require.Nil(t, gameServer.shutdownTimer)

	gameServer.scheduleShutdown()

	require.NotNil(t, gameServer.shutdownTimer)
}

func TestShutdown(t *testing.T) {
	_, mockBusSession, mockGfClient, gameServer := createTestGameServer(t)
	require.NotEmpty(t, mockBusSession)
	require.NotEmpty(t, mockGfClient)
	require.NotEmpty(t, gameServer)

	client := &Client{
		userID:     uuid.New(),
		connID:     uuid.New(),
		gameServer: gameServer,
		send:       make(chan OutgoingMessage, ClientSendChanBufSize),
	}

	mockGfClient.EXPECT().
		AddPlayers(gomock.Eq(gameServer.serverID), gomock.Eq([]uuid.UUID{client.userID})).
		Times(1).
		Return()

	gameServer.addClient(client)

	require.False(t, gameServer.IsClosed())
	require.Len(t, gameServer.clients, 1)

	mockGfClient.EXPECT().
		RemovePlayers(gomock.Eq(gameServer.serverID), gomock.Eq([]uuid.UUID{client.userID})).
		Times(1).
		Return()

	mockGfClient.EXPECT().
		UnregisterGameServerInstance(gomock.Eq(gameServer.serverID), gomock.Eq(gameServer.instanceID)).
		Times(1).
		Return(nil)

	mockGfClient.EXPECT().
		Flush().
		Times(1).
		Return()

	mockBusSession.EXPECT().
		Close().
		Times(1).
		Return()

	gameServer.shutdown()

	require.True(t, gameServer.IsClosed())
	require.Len(t, gameServer.clients, 0)
	require.Equal(t, gameServer.clientCount.Load(), int32(0))
	require.Equal(t, gameServer.connCount.Load(), int32(0))
}

func TestCancelScheduledShutdown(t *testing.T) {
	_, _, _, gameServer := createTestGameServer(t)
	require.NotEmpty(t, gameServer)

	gameServer.scheduleShutdown()
	require.NotNil(t, gameServer.shutdownTimer)

	isCancelled := gameServer.cancelScheduledShutdown()
	require.Nil(t, gameServer.shutdownTimer)
	require.True(t, isCancelled)
}

func TestCancelScheduledShutdown_TimerAlreadyTriggered(t *testing.T) {
	_, mockBusSession, mockGfClient, gameServer := createTestGameServer(t)
	require.NotEmpty(t, mockBusSession)
	require.NotEmpty(t, gameServer)

	gameServer.scheduleShutdown()
	require.NotNil(t, gameServer.shutdownTimer)

	mockBusSession.EXPECT().
		Close().
		Times(1).
		Return()

	mockGfClient.EXPECT().
		UnregisterGameServerInstance(gomock.Eq(gameServer.serverID), gomock.Eq(gameServer.instanceID)).
		Times(1).
		Return(nil)

	mockGfClient.EXPECT().
		Flush().
		Times(1).
		Return()

	// Trigger timer
	gameServer.shutdownTimer.Reset(0)
	time.Sleep(time.Millisecond * 50)

	isCancelled := gameServer.cancelScheduledShutdown()
	require.False(t, isCancelled)
}

func TestGetServerId(t *testing.T) {
	_, _, _, gameServer := createTestGameServer(t)
	require.NotEmpty(t, gameServer)

	retrievedServerId := gameServer.GetServerId()

	require.Equal(t, gameServer.serverID, retrievedServerId)
}

func TestBroadcast(t *testing.T) {
	_, _, _, gameServer := createTestGameServer(t)
	require.NotEmpty(t, gameServer)

	received := make(chan []byte)

	go func() {
		select {
		case msg := <-gameServer.broadcast:
			received <- msg
		case <-time.After(2 * time.Second):
			require.Fail(t, "timed out waiting for broadcast receive goroutine")
		}
	}()

	testMsg := []byte("test msg")
	err := gameServer.Broadcast(testMsg)
	require.NoError(t, err)

	select {
	case retrievedMsg := <-received:
		require.Equal(t, testMsg, retrievedMsg)
	case <-time.After(2 * time.Second):
		require.Fail(t, "timed out waiting for broadcast")
	}
}

func TestSendDirectMsg(t *testing.T) {
	_, _, _, gameServer := createTestGameServer(t)
	require.NotEmpty(t, gameServer)

	received := make(chan *DirectMsgPayload)

	go func() {
		select {
		case msg := <-gameServer.directMsg:
			received <- msg
		case <-time.After(2 * time.Second):
			require.Fail(t, "timed out waiting for directMsg receive goroutine")
		}
	}()

	payload := &DirectMsgPayload{
		Recipients: []DirectMsgRecipient{
			{
				UserID: uuid.New(),
				ConnIDs: []uuid.UUID{
					uuid.New(),
				},
			},
		},
	}
	err := gameServer.SendDirectMsg(payload)
	require.NoError(t, err)

	select {
	case retrievedPayload := <-received:
		require.Equal(t, payload, retrievedPayload)
	case <-time.After(2 * time.Second):
		require.Fail(t, "timed out waiting for sendDirectMsg")
	}
}

func TestRegister(t *testing.T) {
	_, _, _, gameServer := createTestGameServer(t)
	require.NotEmpty(t, gameServer)

	received := make(chan *Client)

	go func() {
		select {
		case client := <-gameServer.register:
			received <- client
		case <-time.After(2 * time.Second):
			require.Fail(t, "timed out waiting for register goroutine")
		}
	}()

	client := &Client{
		userID:     uuid.New(),
		connID:     uuid.New(),
		gameServer: gameServer,
		send:       make(chan OutgoingMessage, ClientSendChanBufSize),
	}

	err := gameServer.Register(client)
	require.NoError(t, err)

	select {
	case retrievedClient := <-received:
		require.Equal(t, client, retrievedClient)
	case <-time.After(2 * time.Second):
		require.Fail(t, "timed out waiting for register")
	}
}

func TestUnregister(t *testing.T) {
	_, _, _, gameServer := createTestGameServer(t)
	require.NotEmpty(t, gameServer)

	received := make(chan *Client)

	go func() {
		select {
		case client := <-gameServer.unregister:
			received <- client
		case <-time.After(2 * time.Second):
			require.Fail(t, "timed out waiting for unregister goroutine")
		}
	}()

	client := &Client{
		userID:     uuid.New(),
		connID:     uuid.New(),
		gameServer: gameServer,
		send:       make(chan OutgoingMessage, ClientSendChanBufSize),
	}

	err := gameServer.Unregister(client)
	require.NoError(t, err)

	select {
	case retrievedClient := <-received:
		require.Equal(t, client, retrievedClient)
	case <-time.After(2 * time.Second):
		require.Fail(t, "timed out waiting for unregister")
	}
}

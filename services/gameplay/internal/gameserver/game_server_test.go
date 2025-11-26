package gameserver

import (
	"context"
	"testing"
	"time"

	mockgameflowclient "github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow/runtime/mock"

	"go.uber.org/mock/gomock"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func createTestGameServer(t *testing.T) *GameServer {
	crtl := gomock.NewController(t)
	defer crtl.Finish()

	gfClient := mockgameflowclient.NewMockClient(crtl)

	serverId := uuid.New()
	gameServer := NewGameServer(context.Background(), serverId, gfClient, &NewGameServerOptions{StartServer: false})

	require.NotEmpty(t, gameServer)
	require.Equal(t, serverId, gameServer.serverId)

	return gameServer
}

func TestCreateGameServer(t *testing.T) {
	gameServer := createTestGameServer(t)
	require.NotEmpty(t, gameServer)
}

func TestAddClient(t *testing.T) {
	gameServer := createTestGameServer(t)
	require.NotEmpty(t, gameServer)

	client := &Client{
		userId:     uuid.New(),
		connId:     uuid.New(),
		gameServer: gameServer,
		send:       make(chan OutgoingMessage, ClientSendChanBufSize),
	}

	require.Len(t, gameServer.clients, 0)
	require.Len(t, gameServer.clients[client.userId], 0)
	require.Equal(t, gameServer.clientCount.Load(), int32(0))
	require.Equal(t, gameServer.connCount.Load(), int32(0))

	gameServer.addClient(client)

	require.Len(t, gameServer.clients, 1)
	require.Len(t, gameServer.clients[client.userId], 1)
	require.Equal(t, gameServer.clientCount.Load(), int32(1))
	require.Equal(t, gameServer.connCount.Load(), int32(1))
}

func TestGetClientConnections(t *testing.T) {
	gameServer := createTestGameServer(t)
	require.NotEmpty(t, gameServer)

	userId := uuid.New()

	clientConns := gameServer.GetClientConnections(userId)
	require.Empty(t, clientConns)
	require.Len(t, clientConns, 0)

	client1 := &Client{
		userId:     userId,
		connId:     uuid.New(),
		gameServer: gameServer,
		send:       make(chan OutgoingMessage, ClientSendChanBufSize),
	}
	gameServer.addClient(client1)

	clientConns = gameServer.GetClientConnections(userId)
	require.NotEmpty(t, clientConns)
	require.Len(t, clientConns, 1)

	client2 := &Client{
		userId:     userId,
		connId:     uuid.New(),
		gameServer: gameServer,
		send:       make(chan OutgoingMessage, ClientSendChanBufSize),
	}
	gameServer.addClient(client2)

	clientConns = gameServer.GetClientConnections(userId)
	require.NotEmpty(t, clientConns)
	require.Len(t, clientConns, 2)
}

func TestRemoveClient(t *testing.T) {
	gameServer := createTestGameServer(t)
	require.NotEmpty(t, gameServer)

	client := &Client{
		userId:     uuid.New(),
		connId:     uuid.New(),
		gameServer: gameServer,
		send:       make(chan OutgoingMessage, ClientSendChanBufSize),
	}

	gameServer.addClient(client)

	require.Len(t, gameServer.clients, 1)
	require.Len(t, gameServer.clients[client.userId], 1)
	require.Equal(t, gameServer.clientCount.Load(), int32(1))
	require.Equal(t, gameServer.connCount.Load(), int32(1))

	gameServer.removeClient(client)

	require.Len(t, gameServer.clients, 0)
	require.Len(t, gameServer.clients[client.userId], 0)
	require.Equal(t, gameServer.clientCount.Load(), int32(0))
	require.Equal(t, gameServer.connCount.Load(), int32(0))
}

func TestRemoveClient_UserIdNotRegistered(t *testing.T) {
	gameServer := createTestGameServer(t)
	require.NotEmpty(t, gameServer)

	client := &Client{
		userId:     uuid.New(),
		connId:     uuid.New(),
		gameServer: gameServer,
		send:       make(chan OutgoingMessage, ClientSendChanBufSize),
	}

	gameServer.addClient(client)

	originalClientUserId := client.userId
	client.userId = uuid.New()
	gameServer.removeClient(client)

	require.Len(t, gameServer.clients, 1)
	require.Len(t, gameServer.clients[originalClientUserId], 1)
	require.Equal(t, gameServer.clientCount.Load(), int32(1))
	require.Equal(t, gameServer.connCount.Load(), int32(1))
}

func TestRemoveClient_UserConnNotRegistered(t *testing.T) {
	gameServer := createTestGameServer(t)
	require.NotEmpty(t, gameServer)

	client := &Client{
		userId:     uuid.New(),
		connId:     uuid.New(),
		gameServer: gameServer,
		send:       make(chan OutgoingMessage, ClientSendChanBufSize),
	}

	gameServer.addClient(client)

	client.connId = uuid.New()
	gameServer.removeClient(client)

	require.Len(t, gameServer.clients, 1)
	require.Len(t, gameServer.clients[client.userId], 1)
	require.Equal(t, gameServer.clientCount.Load(), int32(1))
	require.Equal(t, gameServer.connCount.Load(), int32(1))
}

func TestRemoveAllClients(t *testing.T) {
	gameServer := createTestGameServer(t)
	require.NotEmpty(t, gameServer)

	client1 := &Client{
		userId:     uuid.New(),
		connId:     uuid.New(),
		gameServer: gameServer,
		send:       make(chan OutgoingMessage, ClientSendChanBufSize),
	}

	client2 := &Client{
		userId:     uuid.New(),
		connId:     uuid.New(),
		gameServer: gameServer,
		send:       make(chan OutgoingMessage, ClientSendChanBufSize),
	}

	gameServer.addClient(client1)
	gameServer.addClient(client2)

	require.Len(t, gameServer.clients, 2)
	require.Len(t, gameServer.clients[client1.userId], 1)
	require.Len(t, gameServer.clients[client2.userId], 1)
	require.Equal(t, gameServer.clientCount.Load(), int32(2))
	require.Equal(t, gameServer.connCount.Load(), int32(2))

	gameServer.removeAllClients()

	require.Len(t, gameServer.clients, 0)
	require.Len(t, gameServer.clients[client1.userId], 0)
	require.Len(t, gameServer.clients[client2.userId], 0)
	require.Equal(t, gameServer.clientCount.Load(), int32(0))
	require.Equal(t, gameServer.connCount.Load(), int32(0))
}

func TestDispatchMsg(t *testing.T) {
	gameServer := createTestGameServer(t)
	require.NotEmpty(t, gameServer)

	client := &Client{
		userId:     uuid.New(),
		connId:     uuid.New(),
		gameServer: gameServer,
		send:       make(chan OutgoingMessage, ClientSendChanBufSize),
	}

	gameServer.addClient(client)

	msg := []byte("test msg")
	gameServer.dispatchMsg(msg)

	retrievedMsg := <-client.send
	require.Equal(t, msg, retrievedMsg.Data)
	require.False(t, retrievedMsg.RequiresAck)
	require.Empty(t, retrievedMsg.CorrelationID)
}

func TestDispatchDirectMsg_RecipientUserIdNotFound(t *testing.T) {
	gameServer := createTestGameServer(t)
	require.NotEmpty(t, gameServer)

	client := &Client{
		userId:     uuid.New(),
		connId:     uuid.New(),
		gameServer: gameServer,
		send:       make(chan OutgoingMessage, ClientSendChanBufSize),
	}

	gameServer.addClient(client)

	testMsg := []byte("test msg")
	payload := &DirectMsgPayload{
		Recipients: []DirectMsgRecipient{
			{
				UserId:  uuid.New(),
				ConnIds: []uuid.UUID{client.connId},
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
	gameServer := createTestGameServer(t)
	require.NotEmpty(t, gameServer)

	client := &Client{
		userId:     uuid.New(),
		connId:     uuid.New(),
		gameServer: gameServer,
		send:       make(chan OutgoingMessage, ClientSendChanBufSize),
	}

	gameServer.addClient(client)

	testMsg := []byte("test msg")
	payload := &DirectMsgPayload{
		Recipients: []DirectMsgRecipient{
			{
				UserId:  client.userId,
				ConnIds: []uuid.UUID{uuid.New()},
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
	gameServer := createTestGameServer(t)
	require.NotEmpty(t, gameServer)

	userId := uuid.New()

	client1 := &Client{
		userId:     userId,
		connId:     uuid.New(),
		gameServer: gameServer,
		send:       make(chan OutgoingMessage, ClientSendChanBufSize),
	}

	client2 := &Client{
		userId:     userId,
		connId:     uuid.New(),
		gameServer: gameServer,
		send:       make(chan OutgoingMessage, ClientSendChanBufSize),
	}

	gameServer.addClient(client1)
	gameServer.addClient(client2)

	testMsg := []byte("test msg")
	payload := &DirectMsgPayload{
		Recipients: []DirectMsgRecipient{
			{
				UserId: userId,
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
	gameServer := createTestGameServer(t)
	require.NotEmpty(t, gameServer)

	userId := uuid.New()

	client1 := &Client{
		userId:     userId,
		connId:     uuid.New(),
		gameServer: gameServer,
		send:       make(chan OutgoingMessage, ClientSendChanBufSize),
	}

	client2 := &Client{
		userId:     userId,
		connId:     uuid.New(),
		gameServer: gameServer,
		send:       make(chan OutgoingMessage, ClientSendChanBufSize),
	}

	client3 := &Client{
		userId:     userId,
		connId:     uuid.New(),
		gameServer: gameServer,
		send:       make(chan OutgoingMessage, ClientSendChanBufSize),
	}

	gameServer.addClient(client1)
	gameServer.addClient(client2)
	gameServer.addClient(client3)

	testMsg := []byte("test msg")
	payload := &DirectMsgPayload{
		Recipients: []DirectMsgRecipient{
			{
				UserId:  userId,
				ConnIds: []uuid.UUID{client2.connId, client3.connId},
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

func TestScheduleShutdown(t *testing.T) {
	gameServer := createTestGameServer(t)
	require.NotEmpty(t, gameServer)

	require.Nil(t, gameServer.shutdownTimer)

	gameServer.scheduleShutdown()

	require.NotNil(t, gameServer.shutdownTimer)
}

func TestShutdown(t *testing.T) {
	gameServer := createTestGameServer(t)
	require.NotEmpty(t, gameServer)

	client := &Client{
		userId:     uuid.New(),
		connId:     uuid.New(),
		gameServer: gameServer,
		send:       make(chan OutgoingMessage, ClientSendChanBufSize),
	}

	gameServer.addClient(client)

	require.False(t, gameServer.IsClosed())
	require.Len(t, gameServer.clients, 1)

	gameServer.shutdown()

	require.True(t, gameServer.IsClosed())
	require.Len(t, gameServer.clients, 0)
	require.Equal(t, gameServer.clientCount.Load(), int32(0))
	require.Equal(t, gameServer.connCount.Load(), int32(0))
}

func TestCancelScheduledShutdown(t *testing.T) {
	gameServer := createTestGameServer(t)
	require.NotEmpty(t, gameServer)

	gameServer.scheduleShutdown()
	require.NotNil(t, gameServer.shutdownTimer)

	isCancelled := gameServer.cancelScheduledShutdown()
	require.Nil(t, gameServer.shutdownTimer)
	require.True(t, isCancelled)
}

func TestCancelScheduledShutdown_TimerAlreadyTriggered(t *testing.T) {
	gameServer := createTestGameServer(t)
	require.NotEmpty(t, gameServer)

	gameServer.scheduleShutdown()
	require.NotNil(t, gameServer.shutdownTimer)

	// Trigger timer
	gameServer.shutdownTimer.Reset(0)
	time.Sleep(time.Millisecond * 50)

	isCancelled := gameServer.cancelScheduledShutdown()
	require.False(t, isCancelled)
}

func TestGetServerId(t *testing.T) {
	gameServer := createTestGameServer(t)
	require.NotEmpty(t, gameServer)

	retrievedServerId := gameServer.GetServerId()

	require.Equal(t, gameServer.serverId, retrievedServerId)
}

func TestBroadcast(t *testing.T) {
	gameServer := createTestGameServer(t)
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
	gameServer := createTestGameServer(t)
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
				UserId: uuid.New(),
				ConnIds: []uuid.UUID{
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
	gameServer := createTestGameServer(t)
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
		userId:     uuid.New(),
		connId:     uuid.New(),
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
	gameServer := createTestGameServer(t)
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
		userId:     uuid.New(),
		connId:     uuid.New(),
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

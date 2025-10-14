package websocketserver

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	commonMiddleware "github.com/spazzle-io/spazzle-api/libs/common/middleware"
	"github.com/stretchr/testify/require"
)

type mockGameServer struct {
	broadcastChan  chan []byte
	unregisterChan chan bool
}

func (gs *mockGameServer) GetServerId() uuid.UUID {
	return uuid.New()
}

func (gs *mockGameServer) Broadcast(msg []byte) error {
	gs.broadcastChan <- msg
	return nil
}

func (gs *mockGameServer) Unregister(_ *Client) error {
	gs.unregisterChan <- true
	return nil
}

func createTestClient(t *testing.T) (*httptest.Server, *websocket.Conn, *Client) {
	clientCh := make(chan *Client, 1)

	sm := createTestGameServerManager(t)
	config := getTestConfig()

	initialStartClientPumps := startClientPumps
	startClientPumps = func(ctx context.Context, c *Client) {}
	defer func() {
		startClientPumps = initialStartClientPumps
	}()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		client, err := ServeWs(context.Background(), sm, config, w, r)
		require.NoError(t, err)
		require.NotEmpty(t, client)

		clientCh <- client
	}))

	userId := uuid.New()
	serverId := uuid.New()

	url := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	url += fmt.Sprintf("?server_id=%s&user_id=%s", serverId, userId)

	header := http.Header{}
	validToken := "test-token-123"
	header.Add(commonMiddleware.AuthorizationHeader, fmt.Sprintf("%s %s", commonMiddleware.AuthorizationBearer, validToken))

	conn, _, err := websocket.DefaultDialer.Dial(url, header)
	require.NoError(t, err)

	client := <-clientCh

	return server, conn, client
}

func TestCreateClient(t *testing.T) {
	server, conn, client := createTestClient(t)
	defer server.Close()
	defer func(conn *websocket.Conn) {
		err := conn.Close()
		require.NoError(t, err)
	}(conn)

	require.NotEmpty(t, client)
}

func TestClientReadPump_BroadcastMessage(t *testing.T) {
	server, conn, client := createTestClient(t)
	defer server.Close()
	defer func(conn *websocket.Conn) {
		err := conn.Close()
		require.NoError(t, err)
	}(conn)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	gameServer := &mockGameServer{
		broadcastChan: make(chan []byte, 1),
	}
	client.gameServer = gameServer

	go client.readPump(ctx)

	testMsg := []byte(`test msg`)
	err := conn.WriteMessage(websocket.TextMessage, testMsg)
	require.NoError(t, err)

	select {
	case msg := <-gameServer.broadcastChan:
		require.Equal(t, testMsg, msg)
	case <-time.After(time.Second):
		t.Errorf("timed out waiting for broadcast message")
	}

	cancel()
}

func TestClientReadPump_HandlesDisconnect(t *testing.T) {
	server, conn, client := createTestClient(t)
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	gameServer := &mockGameServer{
		unregisterChan: make(chan bool, 1),
	}
	client.gameServer = gameServer

	go client.readPump(ctx)

	err := conn.Close()
	require.NoError(t, err)

	select {
	case <-gameServer.unregisterChan:
	// Success. Client was unregistered
	case <-time.After(time.Second):
		t.Errorf("timed out waiting for unregister")
	}
}

func TestClientWritePump_ReceiveMessage(t *testing.T) {
	server, conn, client := createTestClient(t)
	defer server.Close()
	defer func(conn *websocket.Conn) {
		err := conn.Close()
		require.NoError(t, err)
	}(conn)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go client.writePump(ctx)

	testMsg := []byte(`test msg`)
	client.send <- testMsg

	_, response, err := conn.ReadMessage()
	require.NoError(t, err)
	require.Equal(t, testMsg, response)
}

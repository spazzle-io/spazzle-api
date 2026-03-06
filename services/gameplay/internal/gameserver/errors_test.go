package gameserver

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestSendError(t *testing.T) {
	mockConfig, gameServer := createInitializedTestGameServer(t)

	client := &Client{
		userID:     uuid.New(),
		connID:     uuid.New(),
		gameServer: gameServer,
		send:       make(chan *OutgoingMessage, ClientSendChanBufSize),
	}

	mockConfig.GfClient.EXPECT().
		AddPlayers(gomock.Eq(gameServer.serverID), gomock.Eq(gameServer.gameID), gomock.Eq([]uuid.UUID{client.userID})).
		Times(1).
		Return()

	gameServer.addClient(client)
	gameServer.sendError(client, ErrCodeServerError, "server error")

	<-client.send // Discarding connection_info msg
	errorMsg := <-client.send

	require.NotNil(t, errorMsg)
	require.Equal(t, TypeClientError, errorMsg.Type)

	var payload ClientError
	err := json.Unmarshal(errorMsg.Payload, &payload)
	require.NoError(t, err)

	require.Equal(t, ErrCodeServerError, payload.Code)
	require.Equal(t, "server error", payload.Message)
}

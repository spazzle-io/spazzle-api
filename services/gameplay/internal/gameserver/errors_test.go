package gameserver

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestSendError(t *testing.T) {
	_, _, _, gameServer := createTestGameServer(t)
	require.NotEmpty(t, gameServer)

	client := createTestClient(t, false)
	require.NotEmpty(t, client)

	directMsgCh := make(chan *DirectMsgPayload, 1)

	go func() {
		select {
		case msg := <-gameServer.directMsg:
			directMsgCh <- msg
		case <-time.After(1 * time.Second):
			directMsgCh <- nil
		}
	}()

	gameServer.sendError(client, ErrCodeServerError, "server error")

	sentMsg := <-directMsgCh

	expectedRecipients := []DirectMsgRecipient{
		{
			UserID:            client.userID,
			ConnIDs:           []uuid.UUID{client.connID},
			ExcludeSpectators: false,
		},
	}

	require.NotNil(t, sentMsg)
	require.Equal(t, TypeClientError, sentMsg.Msg.Type)
	require.Equal(t, expectedRecipients, sentMsg.Recipients)

	var payload ClientError
	err := json.Unmarshal(sentMsg.Msg.Payload, &payload)
	require.NoError(t, err)

	require.Equal(t, ErrCodeServerError, payload.Code)
	require.Equal(t, "server error", payload.Message)
}

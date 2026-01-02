package gameserver

import (
	"encoding/json"

	"github.com/google/uuid"
)

type ClientErrorCode string

const (
	TypeClientError = "error"

	ErrCodeInvalidRequest ClientErrorCode = "invalid_request"
	ErrCodeNotFound       ClientErrorCode = "not_found"
	ErrCodeServerError    ClientErrorCode = "server_error"
)

type ClientError struct {
	Code    ClientErrorCode `json:"code"`
	Message string          `json:"message,omitempty"`
}

func (gs *GameServer) sendError(c *Client, code ClientErrorCode, message string) {
	payload, err := json.Marshal(ClientError{
		Code:    code,
		Message: message,
	})
	if err != nil {
		gs.getLogger(gs.getGameID(), c).Error().Err(err).Msg("could not marshal client error payload")
		return
	}

	err = gs.SendDirectMsg(&DirectMsgPayload{
		Recipients: []DirectMsgRecipient{
			{
				UserID:  c.userID,
				ConnIDs: []uuid.UUID{c.connID},
			},
		},
		Msg: &OutgoingMessage{
			WsMessage: WsMessage{
				Type:    TypeClientError,
				Payload: payload,
			},
			RequiresWorkflowAck: false,
		},
	})
	if err != nil {
		gs.getLogger(gs.getGameID(), c).Error().Err(err).Msg("error sending ws client error message")
	}
}

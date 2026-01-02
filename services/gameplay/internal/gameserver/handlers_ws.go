package gameserver

import (
	"encoding/json"
	"errors"

	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameevents"
)

func (gs *GameServer) handleClientWsMessage(c *Client, msg []byte) {
	var wsMessage WsMessage
	if err := json.Unmarshal(msg, &wsMessage); err != nil {
		gs.getLogger(gs.getGameID(), c).Warn().Err(err).Msg("could not unmarshal client ws message")
		return
	}

	switch wsMessage.Type {
	case gameevents.TypeGetWordChoices:
		gs.handleGetWordChoices(c, &wsMessage)
	case gameevents.TypeSelectWord:
		gs.handleSelectWord(c, &wsMessage)
	}
}

func (gs *GameServer) handleGetWordChoices(c *Client, msg *WsMessage) {
	if !gs.isCurrentArtist(c) {
		return
	}

	words, err := gs.getWordChoices()
	if err != nil {
		gs.getLogger(gs.getGameID(), c).Error().Err(err).Msg("could not get word choices")
		gs.sendError(c, ErrCodeServerError, "failed to get word choices")
		return
	}

	payload, err := json.Marshal(gameevents.GetWordChoicesPayload{
		Words: words,
	})
	if err != nil {
		gs.getLogger(gs.getGameID(), c).Error().Err(err).Msg("could not marshal word choices")
		gs.sendError(c, ErrCodeServerError, "")
		return
	}

	err = gs.SendDirectMsg(&DirectMsgPayload{
		Recipients: []DirectMsgRecipient{{
			UserID:            gs.getCurrentArtist(),
			ExcludeSpectators: true,
		}},
		Msg: &OutgoingMessage{
			WsMessage: WsMessage{
				Type:    msg.Type,
				Payload: payload,
			},
			RequiresWorkflowAck: false,
		},
	})
	if err != nil {
		gs.getLogger(gs.getGameID(), c).Error().Err(err).Msg("failed to send word choices to client")
		gs.sendError(c, ErrCodeServerError, "failed to send word choices")
	}
}

func (gs *GameServer) handleSelectWord(c *Client, msg *WsMessage) {
	if !gs.isCurrentArtist(c) {
		return
	}

	var payload gameevents.SelectWordPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		gs.getLogger(gs.getGameID(), c).Error().Err(err).Msg("could not unmarshal select word payload")
		gs.sendError(c, ErrCodeInvalidRequest, "invalid payload")
		return
	}

	if err := gs.chooseWord(payload.Word); err != nil {
		gs.getLogger(gs.getGameID(), c).Error().Err(err).Msg("could not choose word")

		switch {
		case errors.Is(err, ErrNoCachedWords):
			gs.sendError(c, ErrCodeNotFound, "no word choices requested")
		case errors.Is(err, ErrWordNotInChoices):
			gs.sendError(c, ErrCodeNotFound, "word not in choices")
		default:
			gs.sendError(c, ErrCodeServerError, "failed to select word")
		}
	}
}

func (gs *GameServer) isCurrentArtist(c *Client) bool {
	if !c.isSpectating && c.userID == gs.getCurrentArtist() {
		return true
	}

	return false
}

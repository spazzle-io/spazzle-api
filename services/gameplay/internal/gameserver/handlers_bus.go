package gameserver

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/eventbus"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameevents"
)

func (gs *GameServer) handleEventBusMessage(_ context.Context, msg eventbus.Message) {
	switch msg.Type {
	case gameevents.TypeArtistSelected:
		gs.handleArtistSelected(msg)
	case gameevents.TypeArtistConfirmed:
		gs.handleArtistConfirmed(msg)
	case gameevents.TypeBeginWordSelection:
		gs.handleBeginWordSelection(msg)
	case gameevents.TypeWordSelected:
		gs.handleWordSelected(msg)
	}
}

func (gs *GameServer) handleArtistSelected(msg eventbus.Message) {
	if !gs.userHasNonSpectatingClients(msg.TargetClientID) {
		ackReason := "selected artist not in game server instance"
		gs.ackGameEvent(msg.CorrelationID, gameevents.AckStatusNotApplicable, ackReason)
		return
	}

	err := gs.SendDirectMsg(&DirectMsgPayload{
		Recipients: []DirectMsgRecipient{{
			UserID:            msg.TargetClientID,
			ExcludeSpectators: true,
		}},
		Msg: &OutgoingMessage{
			WsMessage: WsMessage{
				ID:         msg.ID,
				Type:       msg.Type,
				Timestamp:  &msg.Timestamp,
				StreamType: msg.StreamType,
				Payload:    msg.Payload,
			},
			CorrelationID:       msg.CorrelationID,
			RequiresWorkflowAck: true,
		},
	})
	if err != nil {
		ackReason := "failed to send artist selected msg to client"
		gs.getLogger(gs.getGameID(), nil).Error().Err(err).Msg(ackReason)
		gs.ackGameEvent(msg.CorrelationID, gameevents.AckStatusFailed, ackReason)
	}
}

func (gs *GameServer) handleArtistConfirmed(msg eventbus.Message) {
	var payload gameevents.ArtistConfirmedPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		gs.getLogger(gs.getGameID(), nil).Error().Err(err).
			Msg("could not unmarshal artist confirmed payload")
		return
	}

	gs.setCurrentArtist(payload.ArtistID)

	err := gs.Broadcast(&WsMessage{
		ID:         msg.ID,
		Type:       msg.Type,
		Timestamp:  &msg.Timestamp,
		StreamType: msg.StreamType,
		Payload:    msg.Payload,
	})
	if err != nil {
		gs.getLogger(gs.getGameID(), nil).Error().Err(err).Msg("failed to broadcast artist confirmed msg")
	}
}

func (gs *GameServer) handleBeginWordSelection(msg eventbus.Message) {
	var payload gameevents.BeginWordSelectionPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		gs.getLogger(gs.getGameID(), nil).Error().Err(err).
			Msg("could not unmarshal begin word selection payload")
		return
	}

	gs.setWorkflowSelectionID(payload.SelectionID)

	err := gs.Broadcast(&WsMessage{
		ID:         msg.ID,
		Type:       msg.Type,
		Timestamp:  &msg.Timestamp,
		StreamType: msg.StreamType,
		Payload:    msg.Payload,
	})
	if err != nil {
		gs.getLogger(gs.getGameID(), nil).Error().Err(err).
			Msg("failed to broadcast begin word selection msg")
	}
}

func (gs *GameServer) handleWordSelected(msg eventbus.Message) {
	gameState, err := gs.GfClient.GetGameState(gs.serverID)
	if err != nil {
		gs.getLogger(gs.getGameID(), nil).Error().Err(err).Msg("failed to get game state")
		return
	}

	gs.setCurrentWord(gameState.CurrentWord.Text)

	err = gs.Broadcast(&WsMessage{
		ID:         msg.ID,
		Type:       msg.Type,
		Timestamp:  &msg.Timestamp,
		StreamType: msg.StreamType,
		Payload:    msg.Payload,
	})
	if err != nil {
		gs.getLogger(gs.getGameID(), nil).Error().Err(err).
			Msg("failed to broadcast begin word selection msg")
	}
}

func (gs *GameServer) userHasNonSpectatingClients(userID uuid.UUID) bool {
	gs.mu.RLock()
	defer gs.mu.RUnlock()

	for _, client := range gs.clients[userID] {
		if !client.isSpectating {
			return true
		}
	}

	return false
}

func (gs *GameServer) setCurrentArtist(currentArtist uuid.UUID) {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	gs.currentArtist = currentArtist
}

func (gs *GameServer) setCurrentWord(currentWord string) {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	gs.currentWord = currentWord
}

func (gs *GameServer) setWorkflowSelectionID(selectionID uuid.UUID) {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	gs.workflowSelectionID = selectionID
}

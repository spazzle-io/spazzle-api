package gameserver

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/eventbus"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameevents"
)

func (gs *GameServer) handleEventBusMessage(_ context.Context, msg eventbus.Message) {
	switch msg.Type {
	case gameevents.TypePlayersJoined:
		gs.handlePlayersJoined(msg)
	case gameevents.TypePlayersLeft:
		gs.handlePlayersLeft(msg)

	case gameevents.TypePlayerWarned:
		gs.broadcastBusMsg(msg)
	case gameevents.TypePlayersReported:
		gs.broadcastBusMsg(msg)
	case gameevents.TypePlayerReportsCleared:
		gs.broadcastBusMsg(msg)
	case gameevents.TypePlayersEjected:
		gs.handlePlayersEjected(msg)

	case gameevents.TypeArtistSelected:
		gs.handleArtistSelected(msg)
	case gameevents.TypeArtistConfirmed:
		gs.handleArtistConfirmed(msg)
	case gameevents.TypeNextArtistSelected:
		gs.handleNextArtistSelected(msg)
	case gameevents.TypeArtistDisconnected:
		gs.handleArtistDisconnected(msg)

	case gameevents.TypeBeginWordSelection:
		gs.broadcastBusMsg(msg)
	case gameevents.TypeWordSelected:
		gs.handleWordSelected(msg)
	case gameevents.TypeWordHintRevealed:
		gs.broadcastBusMsg(msg)
	case gameevents.TypeWordGuessed:
		gs.broadcastBusMsg(msg)

	case gameevents.TypeBeginDrawing:
		gs.broadcastBusMsg(msg)
	case gameevents.TypeEndDrawing:
		gs.broadcastBusMsg(msg)

	case gameevents.TypeRoundEnded:
		gs.handleRoundEnded(msg)
	case gameevents.TypeGameEnded:
		gs.handleGameEnded(msg)
	}
}

func (gs *GameServer) handlePlayersJoined(msg eventbus.Message) {
	logger := gs.logger()

	var payload gameevents.PlayersJoinedPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		logger.Error().Err(err).Msg("could not unmarshal players joined payload")
		return
	}

	gs.addActivePlayers(payload.AddedPlayers)
	for _, rejectedPlayer := range payload.RejectedPlayers {
		gs.removeActivePlayers([]uuid.UUID{rejectedPlayer.PlayerID})
	}

	gs.broadcastBusMsg(msg)
}

func (gs *GameServer) handlePlayersLeft(msg eventbus.Message) {
	logger := gs.logger()

	var payload gameevents.PlayersLeftPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		logger.Error().Err(err).Msg("could not unmarshal players left payload")
		return
	}

	gs.removeActivePlayers(payload.PlayerIDs)
	gs.broadcastBusMsg(msg)
}

func (gs *GameServer) handlePlayersEjected(msg eventbus.Message) {
	logger := gs.logger()

	var payload gameevents.PlayersEjectedPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		logger.Error().Err(err).Msg("could not unmarshal player ejected payload")
		return
	}

	for _, ejection := range payload.Ejections {
		gs.removeActivePlayers([]uuid.UUID{ejection.PlayerID})
	}

	gs.broadcastBusMsg(msg)
}

func (gs *GameServer) handleArtistSelected(msg eventbus.Message) {
	if !gs.userHasNonSpectatingClients(msg.TargetClientID) {
		ackReason := "selected artist not in game server instance"
		gs.ackGameEvent(msg.CorrelationID, gameevents.AckStatusNotApplicable, ackReason)
		return
	}

	gs.sendBusMsgToTargetClient(msg)
}

func (gs *GameServer) handleArtistConfirmed(msg eventbus.Message) {
	logger := gs.logger()

	var payload gameevents.ArtistConfirmedPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		logger.Error().Err(err).Msg("could not unmarshal artist confirmed payload")
		return
	}

	gs.setCurrentArtist(payload.ArtistID)
	gs.setCurrentRound(payload.CurrentRound)
	gs.broadcastBusMsg(msg)
}

func (gs *GameServer) handleNextArtistSelected(msg eventbus.Message) {
	if !gs.userHasNonSpectatingClients(msg.TargetClientID) {
		ackReason := "next artist selected not in game server instance"
		gs.ackGameEvent(msg.CorrelationID, gameevents.AckStatusNotApplicable, ackReason)
		return
	}

	gs.sendBusMsgToTargetClient(msg)
}

func (gs *GameServer) handleArtistDisconnected(msg eventbus.Message) {
	logger := gs.logger()

	var payload gameevents.ArtistDisconnectedPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		logger.Error().Err(err).Msg("could not unmarshal artist disconnected payload")
		return
	}

	gs.setCurrentArtist(uuid.Nil)
	gs.removeActivePlayers([]uuid.UUID{payload.ArtistID})
	gs.broadcastBusMsg(msg)
}

func (gs *GameServer) handleWordSelected(msg eventbus.Message) {
	logger := gs.logger()

	gameState, err := gs.GfClient.GetGameState(gs.serverID)
	if err != nil {
		logger.Error().Err(err).Msg("failed to get game state")
		return
	}

	gs.setCurrentWord(gameState.CurrentWord.Text)
	gs.broadcastBusMsg(msg)
}

func (gs *GameServer) handleRoundEnded(msg eventbus.Message) {
	if msg.StreamType != eventbus.GameEventsStreamType {
		return
	}

	gs.setCurrentArtist(uuid.Nil)
	gs.setCurrentWord("")
	gs.clearCorrectGuessers()

	gs.broadcastBusMsg(msg)
}

func (gs *GameServer) handleGameEnded(msg eventbus.Message) {
	logger := gs.logger()

	gs.broadcastBusMsg(msg)
	gs.isGameActive.Store(false)

	gs.mu.Lock()
	if gs.busSession != nil {
		gs.busSession.Close()
		gs.busSession = nil
	}
	gs.mu.Unlock()

	gs.clearActivePlayers()

	logger.Info().Msg("game ended. scheduling shutdown")
	gs.scheduleShutdown()
}

func (gs *GameServer) broadcastBusMsg(msg eventbus.Message) {
	logger := gs.logger()

	err := gs.Broadcast(&WsMessage{
		ID:         msg.ID,
		Type:       msg.Type,
		Timestamp:  &msg.Timestamp,
		StreamType: msg.StreamType,
		Payload:    msg.Payload,
	})
	if err != nil {
		logger.Error().Err(err).Str("type", msg.Type).Msg("failed to broadcast bus msg")
	}
}

func (gs *GameServer) sendBusMsgToTargetClient(msg eventbus.Message) {
	logger := gs.logger()

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
		ackReason := fmt.Sprintf("failed to send %s msg to client", msg.Type)
		logger.Error().Err(err).Msg(ackReason)
		gs.ackGameEvent(msg.CorrelationID, gameevents.AckStatusFailed, ackReason)
	}
}

func (gs *GameServer) getUserClients(userID uuid.UUID, includeSpectators bool) []*Client {
	gs.mu.RLock()
	defer gs.mu.RUnlock()

	clients := make([]*Client, 0)
	for _, client := range gs.clients[userID] {
		if !client.isSpectating || includeSpectators {
			clients = append(clients, client)
		}
	}

	return clients
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
	previousArtist := gs.currentArtist
	gs.currentArtist = currentArtist
	gs.mu.Unlock()

	if previousArtist != uuid.Nil {
		for _, client := range gs.getUserClients(previousArtist, false) {
			client.UpdateTiming(DefaultTiming)
		}
	}

	if currentArtist != uuid.Nil {
		for _, client := range gs.getUserClients(currentArtist, false) {
			client.UpdateTiming(AggressiveTiming)
		}
	}
}

func (gs *GameServer) setCurrentRound(currentRound uint8) {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	gs.currentRound = currentRound
}

func (gs *GameServer) setCurrentWord(currentWord string) {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	gs.currentWord = currentWord
}

func (gs *GameServer) addActivePlayers(playerIDs []uuid.UUID) {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	for _, playerID := range playerIDs {
		gs.activePlayers[playerID] = true
	}
}

func (gs *GameServer) removeActivePlayers(playerIDs []uuid.UUID) {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	for _, playerID := range playerIDs {
		delete(gs.activePlayers, playerID)
	}
}

func (gs *GameServer) clearActivePlayers() {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	gs.activePlayers = make(map[uuid.UUID]bool)
}

func (gs *GameServer) clearCorrectGuessers() {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	gs.correctGuessers = make(map[uuid.UUID]bool)
}

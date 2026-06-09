package gameserver

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	commonUtil "github.com/spazzle-io/spazzle-api/libs/common/util"
	db "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/sqlc"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/eventbus"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameevents"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow/types"
)

func (gs *GameServer) handleClientWsMessage(c *Client, msg []byte) {
	logger := gs.loggerWithClient(c)

	var wsMessage WsMessage
	if err := json.Unmarshal(msg, &wsMessage); err != nil {
		logger.Warn().Err(err).Msg("could not unmarshal client ws message")
		return
	}

	if !gs.isGameActive.Load() {
		logger.Warn().
			Str("type", wsMessage.Type).
			Msg("could not process client ws message. Game is not active")
		return
	}

	switch wsMessage.Type {
	case gameevents.TypeJoinGame:
		gs.handleJoinGame(c, &wsMessage)

	case gameevents.TypeUpdateDrawing:
		gs.handleUpdateDrawing(c, &wsMessage)

	case gameevents.TypeGetWordChoices:
		gs.handleGetWordChoices(c, &wsMessage)
	case gameevents.TypeSelectWord:
		gs.handleSelectWord(c, &wsMessage)
	case gameevents.TypeGuessWord:
		gs.handleGuessWord(c, &wsMessage)

	case gameevents.TypeWarnPlayer:
		gs.handleWarnPlayer(c, &wsMessage)
	case gameevents.TypeReportPlayer:
		gs.handleReportPlayer(c, &wsMessage)
	case gameevents.TypeClearPlayerReports:
		gs.handleClearPlayerReports(c, &wsMessage)
	case gameevents.TypeEjectPlayer:
		gs.handleEjectPlayer(c, &wsMessage)
	}
}

func (gs *GameServer) handleJoinGame(c *Client, msg *WsMessage) {
	logger := gs.loggerWithClient(c)

	var payload gameevents.JoinGamePayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		logger.Error().Err(err).Msg("could not unmarshal join game payload")
		gs.sendError(c, ErrCodeInvalidRequest, "invalid payload")
		return
	}

	if gs.IsClosed() {
		logger.Error().Msg("game server is closed")
		gs.sendError(c, ErrCodeJoinError, "could not join game")
		return
	}

	if !gs.IsGameActive() {
		logger.Error().Msg("game server is not active")
		gs.sendError(c, ErrCodeJoinError, "could not join game")
		return
	}

	joinCodeEntry, err := gs.GameCache.ValidateJoinCode(
		context.Background(), payload.JoinCode, gs.GetServerId(), gs.GetGameID(),
	)
	if err != nil {
		logger.Error().Err(err).Msg("could not validate join code")
		gs.sendError(c, ErrCodeJoinError, "could not join game")
		return
	}

	if joinCodeEntry.UserID != c.userID {
		logger.Warn().
			Str("expected_user_id", joinCodeEntry.UserID.String()).
			Str("actual_user_id", c.userID.String()).
			Msg("join code user mismatch")
		gs.sendError(c, ErrCodeJoinError, "could not join game")
		return
	}

	role, err := ParseRole(joinCodeEntry.Role)
	if err != nil {
		logger.Error().Err(err).Msg("failed to parse role")
		gs.sendError(c, ErrCodeJoinError, "could not join game")
		return
	}

	err = gs.Register(c)
	if err != nil {
		gs.sendError(c, ErrCodeJoinError, "could not join game")
		return
	}

	if err := gs.GameCache.InvalidateJoinCode(context.Background(), payload.JoinCode); err != nil {
		logger.Warn().Err(err).Msg("failed to invalidate join code")
	}

	c.role.Store(role)
}

func (gs *GameServer) handleUpdateDrawing(c *Client, _ *WsMessage) {
	if !gs.isCurrentArtist(c) {
		return
	}

	// TODO: Validate msg, publish to drawing updates stream, and broadcast to all players.
}

func (gs *GameServer) handleGetWordChoices(c *Client, msg *WsMessage) {
	if !gs.isCurrentArtist(c) {
		return
	}

	if strings.TrimSpace(gs.currentWord) != "" {
		return
	}

	logger := gs.loggerWithClient(c)

	words, err := gs.getWordChoices()
	if err != nil {
		logger.Error().Err(err).Msg("could not get word choices")
		gs.sendError(c, ErrCodeServerError, "failed to get word choices")
		return
	}

	payload, err := json.Marshal(gameevents.GetWordChoicesPayload{
		Words: words,
	})
	if err != nil {
		logger.Error().Err(err).Msg("could not marshal word choices")
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
		logger.Error().Err(err).Msg("failed to send word choices to client")
		gs.sendError(c, ErrCodeServerError, "failed to send word choices")
	}
}

func (gs *GameServer) handleSelectWord(c *Client, msg *WsMessage) {
	if !gs.isCurrentArtist(c) {
		return
	}

	logger := gs.loggerWithClient(c)

	var payload gameevents.SelectWordPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		logger.Error().Err(err).Msg("could not unmarshal select word payload")
		gs.sendError(c, ErrCodeInvalidRequest, "invalid payload")
		return
	}

	if strings.TrimSpace(payload.Word) == "" {
		gs.sendError(c, ErrCodeNotFound, "cannot select empty word")
		return
	}

	if err := gs.chooseWord(payload.Word); err != nil {
		logger.Error().Err(err).Msg("could not choose word")

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

func (gs *GameServer) handleGuessWord(c *Client, msg *WsMessage) {
	if gs.isCurrentArtist(c) || !gs.isActivePlayer(c.userID) || !c.IsPlayer() {
		return
	}

	logger := gs.loggerWithClient(c)

	var payload gameevents.GuessWordPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		logger.Error().Err(err).Msg("could not unmarshal guess word payload")
		gs.sendError(c, ErrCodeInvalidRequest, "invalid payload")
		return
	}

	if strings.TrimSpace(payload.Guess) == "" {
		return
	}

	canGuess := !gs.isCorrectGuesser(c.userID)
	matchesWord := commonUtil.EqualText(payload.Guess, gs.getCurrentWord())
	isCorrectGuess := canGuess && matchesWord

	if !canGuess && matchesWord {
		return
	}

	if isCorrectGuess {
		gs.GfClient.RecordCorrectGuesses(gs.serverID, gs.GetGameID(), gs.getCurrentRound(), []types.CorrectGuess{
			{
				PlayerID:  c.userID,
				Timestamp: time.Now().UTC(),
			},
		})
	}

	broadcastedGuess := payload.Guess
	if matchesWord {
		broadcastedGuess = ""
	}

	wordGuessedPayload, err := json.Marshal(&gameevents.WordGuessedPayload{
		Guess:        broadcastedGuess,
		IsCorrect:    isCorrectGuess,
		PlayerID:     c.userID,
		CurrentRound: gs.getCurrentRound(),
	})
	if err != nil {
		logger.Error().Err(err).Msg("could not marshal word guessed payload")
		gs.sendError(c, ErrCodeServerError, "internal server error")
		return
	}

	_, err = gs.busSession.Publish(gs.ctx, eventbus.GameEventsStreamType, eventbus.PublishMessage{
		Type:    gameevents.TypeWordGuessed,
		Payload: wordGuessedPayload,
	})
	if err != nil {
		logger.Error().Err(err).Msg("could not publish word guessed payload")
		gs.sendError(c, ErrCodeServerError, "could not record guess")
		return
	}

	if isCorrectGuess {
		gs.addCorrectGuesser(c.userID)
	}
}

func (gs *GameServer) handleWarnPlayer(c *Client, msg *WsMessage) {
	logger := gs.loggerWithClient(c)

	var payload gameevents.WarnPlayerPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		logger.Error().Err(err).Msg("could not unmarshal warn player payload")
		gs.sendError(c, ErrCodeInvalidRequest, "invalid payload")
		return
	}

	permissions, err := gs.Store.GetServerUserPermissions(gs.ctx, db.GetServerUserPermissionsParams{
		ServerID: gs.serverID,
		UserID:   c.userID,
	})
	if err != nil {
		logger.Error().Err(err).Msg("could not get server user permissions")
		gs.sendError(c, ErrCodeServerError, "internal server error")
		return
	}

	if !permissions.HasElevatedPermissions {
		return
	}

	playerWarnedPayload, err := json.Marshal(&gameevents.PlayerWarnedPayload{
		PlayerID: payload.PlayerID,
	})
	if err != nil {
		logger.Error().Err(err).Msg("could not marshal player warned payload")
		gs.sendError(c, ErrCodeServerError, "internal server error")
		return
	}

	_, err = gs.busSession.Publish(gs.ctx, eventbus.GameEventsStreamType, eventbus.PublishMessage{
		Type:    gameevents.TypePlayerWarned,
		Payload: playerWarnedPayload,
	})
	if err != nil {
		logger.Error().Err(err).Msg("could not publish player warned payload")
		gs.sendError(c, ErrCodeServerError, "could not warn player")
	}
}

func (gs *GameServer) handleReportPlayer(c *Client, msg *WsMessage) {
	if !gs.isActivePlayer(c.userID) || !c.IsPlayer() {
		return
	}

	logger := gs.loggerWithClient(c)

	var payload gameevents.ReportPlayerPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		logger.Error().Err(err).Msg("could not unmarshal report player payload")
		gs.sendError(c, ErrCodeInvalidRequest, "invalid payload")
		return
	}

	gs.GfClient.ReportPlayer(gs.serverID, c.userID, payload.ReportTarget)
}

func (gs *GameServer) handleClearPlayerReports(c *Client, msg *WsMessage) {
	logger := gs.loggerWithClient(c)

	var payload gameevents.ClearPlayerReportsPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		logger.Error().Err(err).Msg("could not unmarshal clear player reports payload")
		gs.sendError(c, ErrCodeInvalidRequest, "invalid payload")
		return
	}

	permissions, err := gs.Store.GetServerUserPermissions(gs.ctx, db.GetServerUserPermissionsParams{
		ServerID: gs.serverID,
		UserID:   c.userID,
	})
	if err != nil {
		logger.Error().Err(err).Msg("could not get server user permissions")
		gs.sendError(c, ErrCodeServerError, "internal server error")
		return
	}

	if !permissions.HasElevatedPermissions {
		return
	}

	gs.GfClient.ClearPlayerReports(gs.serverID, payload.PlayerID)
}

func (gs *GameServer) handleEjectPlayer(c *Client, msg *WsMessage) {
	logger := gs.loggerWithClient(c)

	var payload gameevents.EjectPlayerPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		logger.Error().Err(err).Msg("could not unmarshal eject player payload")
		gs.sendError(c, ErrCodeInvalidRequest, "invalid payload")
		return
	}

	permissions, err := gs.Store.GetServerUserPermissions(gs.ctx, db.GetServerUserPermissionsParams{
		ServerID: gs.serverID,
		UserID:   c.userID,
	})
	if err != nil {
		logger.Error().Err(err).Msg("could not get server user permissions")
		gs.sendError(c, ErrCodeServerError, "internal server error")
		return
	}

	if !permissions.HasElevatedPermissions {
		return
	}

	gs.GfClient.EjectPlayer(gs.serverID, payload.PlayerID, c.userID)
}

func (gs *GameServer) sendConnectionInfoMsg(c *Client) {
	logger := gs.loggerWithClient(c)

	payload, err := json.Marshal(&gameevents.ConnectionInfoPayload{
		ServerID:     gs.serverID,
		GameID:       gs.GetGameID(),
		CurrentRound: gs.getCurrentRound(),
		UserID:       c.userID,
		ConnID:       c.connID,
		Role:         string(c.Role()),
	})
	if err != nil {
		logger.Error().Err(err).Msg("failed to marshal player connected payload")
		return
	}

	gs.dispatchDirectMsg(&DirectMsgPayload{
		Recipients: []DirectMsgRecipient{{
			UserID:  c.userID,
			ConnIDs: []uuid.UUID{c.connID},
		}},
		Msg: &OutgoingMessage{
			WsMessage: WsMessage{
				Type:    gameevents.TypeConnectionInfo,
				Payload: payload,
			},
			RequiresWorkflowAck: false,
		},
	})
}

func (gs *GameServer) isCurrentArtist(c *Client) bool {
	if c.IsPlayer() && c.userID == gs.getCurrentArtist() {
		return true
	}

	return false
}

func (gs *GameServer) addCorrectGuesser(id uuid.UUID) {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	gs.correctGuessers[id] = true
}

func (gs *GameServer) isCorrectGuesser(id uuid.UUID) bool {
	gs.mu.RLock()
	defer gs.mu.RUnlock()

	return gs.correctGuessers[id]
}

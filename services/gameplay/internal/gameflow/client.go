package gameflow

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.temporal.io/api/serviceerror"

	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameevents"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow/internal/workflow"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow/types"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/util"
	"go.temporal.io/api/enums/v1"
	temporalclient "go.temporal.io/sdk/client"
)

var ErrGameEnding = errors.New("game ending")

const flushInterval = 3 * time.Second

type signalBuffer struct {
	addPlayers           map[uuid.UUID]map[uuid.UUID][]uuid.UUID
	removePlayers        map[uuid.UUID][]uuid.UUID
	recordCorrectGuesses map[uuid.UUID]map[uuid.UUID]map[uint8][]types.CorrectGuess
	reportPlayers        map[uuid.UUID][]types.PlayerReport
	clearPlayerReports   map[uuid.UUID][]uuid.UUID
	ejectPlayers         map[uuid.UUID][]types.PlayerEjection
}

type clientOptions struct {
	disableAutoFlush bool
}

type ClientOption func(*clientOptions)

func WithoutAutoFlush() ClientOption {
	return func(o *clientOptions) {
		o.disableAutoFlush = true
	}
}

type client struct {
	temporal temporalclient.Client

	mu           sync.RWMutex
	signalBuffer signalBuffer

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	workflowRunByGameID map[uuid.UUID]string
}

var temporalDial = temporalclient.Dial

func NewClient(config *util.Config, opts ...ClientOption) (Client, error) {
	options := &clientOptions{}
	for _, opt := range opts {
		opt(options)
	}

	tc, err := temporalDial(getTemporalClientOpts(config))
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	c := &client{
		temporal: tc,
		signalBuffer: signalBuffer{
			addPlayers:           make(map[uuid.UUID]map[uuid.UUID][]uuid.UUID),
			removePlayers:        make(map[uuid.UUID][]uuid.UUID),
			recordCorrectGuesses: make(map[uuid.UUID]map[uuid.UUID]map[uint8][]types.CorrectGuess),
			reportPlayers:        make(map[uuid.UUID][]types.PlayerReport),
			clearPlayerReports:   make(map[uuid.UUID][]uuid.UUID),
			ejectPlayers:         make(map[uuid.UUID][]types.PlayerEjection),
		},
		ctx:                 ctx,
		cancel:              cancel,
		workflowRunByGameID: make(map[uuid.UUID]string),
	}

	if !options.disableAutoFlush {
		c.wg.Add(1)
		go c.flushSignalBuffer()
	}

	return c, nil
}

func (c *client) Game(gameServerID uuid.UUID, input types.GameInput) (gameID uuid.UUID, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	run, err := c.temporal.ExecuteWorkflow(
		c.ctx,
		temporalclient.StartWorkflowOptions{
			ID:                       gameServerID.String(),
			TaskQueue:                types.GameWorkflowTaskQueue,
			WorkflowIDConflictPolicy: enums.WORKFLOW_ID_CONFLICT_POLICY_USE_EXISTING,
		},
		workflow.GameWorkflow,
		input,
	)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to execute game workflow: %w", err)
	}

	gameState, err := c.GetGameState(gameServerID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to get game state: %w", err)
	}

	c.workflowRunByGameID[gameState.GameID] = run.GetRunID()

	if gameState.Phase == types.PhaseEndGame {
		return gameState.GameID, ErrGameEnding
	}

	return gameState.GameID, nil
}

func (c *client) GetGameState(gameServerID uuid.UUID) (state *types.GameStateView, err error) {
	response, err := c.temporal.QueryWorkflow(c.ctx, gameServerID.String(), "", workflow.QueryGetGameState)
	if err != nil {
		return nil, fmt.Errorf("failed to query workflow for game state: %w", err)
	}

	var gameStateView types.GameStateView
	if err := response.Get(&gameStateView); err != nil {
		return nil, fmt.Errorf("failed to extract queried game state: %w", err)
	}

	return &gameStateView, nil
}

func (c *client) AddPlayers(gameServerID uuid.UUID, gameID uuid.UUID, playerIDs []uuid.UUID) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.signalBuffer.addPlayers[gameServerID] == nil {
		c.signalBuffer.addPlayers[gameServerID] = make(map[uuid.UUID][]uuid.UUID)
	}
	c.signalBuffer.addPlayers[gameServerID][gameID] = append(c.signalBuffer.addPlayers[gameServerID][gameID], playerIDs...)
}

func (c *client) RemovePlayers(gameServerID uuid.UUID, playerIDs []uuid.UUID) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.signalBuffer.removePlayers[gameServerID] = append(c.signalBuffer.removePlayers[gameServerID], playerIDs...)
}

func (c *client) ReportPlayer(gameServerID uuid.UUID, reporter uuid.UUID, reportTarget uuid.UUID) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.signalBuffer.reportPlayers[gameServerID] = append(c.signalBuffer.reportPlayers[gameServerID], types.PlayerReport{
		Reporter:     reporter,
		ReportTarget: reportTarget,
	})
}

func (c *client) ClearPlayerReports(gameServerID uuid.UUID, playerID uuid.UUID) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.signalBuffer.clearPlayerReports[gameServerID] = append(c.signalBuffer.clearPlayerReports[gameServerID], playerID)
}

func (c *client) EjectPlayer(gameServerID uuid.UUID, playerID uuid.UUID, ejector uuid.UUID) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.signalBuffer.ejectPlayers[gameServerID] = append(c.signalBuffer.ejectPlayers[gameServerID], types.PlayerEjection{
		PlayerID: playerID,
		Ejector:  ejector,
	})
}

func (c *client) RecordCorrectGuesses(
	gameServerID uuid.UUID,
	gameID uuid.UUID,
	currentRound uint8,
	correctGuesses []types.CorrectGuess,
) {
	c.mu.Lock()
	defer c.mu.Unlock()

	slot := ensureRecordCorrectGuessesSlot(c.signalBuffer.recordCorrectGuesses, gameServerID, gameID)
	slot[currentRound] = append(slot[currentRound], correctGuesses...)
}

func (c *client) SelectWord(gameServerID uuid.UUID, gameID uuid.UUID, currentRound uint8, word string) error {
	return c.temporal.SignalWorkflow(
		c.ctx,
		gameServerID.String(),
		"",
		workflow.SignalWordSelected,
		workflow.WordSelectedSignal{
			GameID:       gameID,
			CurrentRound: currentRound,
			Word:         word,
		},
	)
}

func (c *client) ArtistDisconnected(gameServerID uuid.UUID, gameID uuid.UUID, currentRound uint8, artistID uuid.UUID) error {
	return c.temporal.SignalWorkflow(
		c.ctx,
		gameServerID.String(),
		"",
		workflow.SignalArtistDisconnected,
		workflow.ArtistDisconnectedSignal{
			GameID:       gameID,
			CurrentRound: currentRound,
			ArtistID:     artistID,
		},
	)
}

func (c *client) HeartbeatGameServerInstance(gameServerID uuid.UUID, gameID uuid.UUID, gameServerInstanceID uuid.UUID) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.temporal.SignalWorkflow(
		c.ctx,
		gameServerID.String(),
		c.workflowRunByGameID[gameID],
		workflow.SignalGameServerInstanceHeartbeat,
		workflow.GameServerInstanceHeartbeatSignal{
			InstanceID: gameServerInstanceID,
		},
	)
}

func (c *client) UnregisterGameServerInstance(gameServerID uuid.UUID, gameServerInstanceID uuid.UUID) error {
	return c.temporal.SignalWorkflow(
		c.ctx,
		gameServerID.String(),
		"",
		workflow.SignalGameServerInstanceUnregistered,
		workflow.GameServerInstanceUnregisteredSignal{
			InstanceID: gameServerInstanceID,
		},
	)
}

func (c *client) AcknowledgeGameEvent(gameServerID uuid.UUID, payload gameevents.EventAckPayload) error {
	return c.temporal.SignalWorkflow(
		c.ctx,
		gameServerID.String(),
		"",
		workflow.SignalEventAck,
		payload,
	)
}

func (c *client) Flush() {
	c.flushAddPlayers()
	c.flushRemovePlayers()
	c.flushRecordCorrectGuesses()
	c.flushReportPlayers()
	c.flushClearPlayerReports()
	c.flushEjectPlayers()
}

func (c *client) ShutdownGameServer(gameServerID uuid.UUID) {
	c.flushAddPlayersForServer(gameServerID)
	c.flushRemovePlayersForServer(gameServerID)
	c.flushRecordCorrectGuessesForServer(gameServerID)
	c.flushReportPlayersForServer(gameServerID)
	c.flushClearPlayerReportsForServer(gameServerID)
	c.flushEjectPlayersForServer(gameServerID)
}

func (c *client) Close() {
	c.cancel()
	c.wg.Wait()
	c.temporal.Close()
}

func (c *client) flushSignalBuffer() {
	defer c.wg.Done()

	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.Flush()
		case <-c.ctx.Done():
			c.Flush()
			return
		}
	}
}

func (c *client) flushAddPlayersForServer(gameServerID uuid.UUID) {
	c.mu.Lock()
	playersPerGame := c.signalBuffer.addPlayers[gameServerID]
	delete(c.signalBuffer.addPlayers, gameServerID)
	c.mu.Unlock()

	for gameID, playerIDs := range playersPerGame {
		if len(playerIDs) == 0 {
			continue
		}

		err := c.temporal.SignalWorkflow(
			c.ctx,
			gameServerID.String(),
			"",
			workflow.SignalPlayersJoin,
			workflow.PlayersJoinSignal{
				GameID:    gameID,
				PlayerIDs: playerIDs,
			},
		)
		if err != nil {
			logger := log.With().Err(err).
				Str("server_id", gameServerID.String()).
				Str("game_id", gameID.String()).
				Int("num_players", len(playerIDs)).
				Any("player_ids", playerIDs).
				Logger()

			if !c.isRetryableError(err) {
				logger.Warn().Msg("dropping addPlayers signal")
				continue
			}

			logger.Error().Msg("failed to flush addPlayers signal buffer")

			c.mu.Lock()
			if c.signalBuffer.addPlayers[gameServerID] == nil {
				c.signalBuffer.addPlayers[gameServerID] = make(map[uuid.UUID][]uuid.UUID)
			}
			c.signalBuffer.addPlayers[gameServerID][gameID] = append(
				c.signalBuffer.addPlayers[gameServerID][gameID], playerIDs...,
			)
			c.mu.Unlock()
		}
	}
}

func (c *client) flushAddPlayers() {
	c.mu.Lock()

	gameServerIDs := make([]uuid.UUID, 0, len(c.signalBuffer.addPlayers))
	for gameServerID := range c.signalBuffer.addPlayers {
		gameServerIDs = append(gameServerIDs, gameServerID)
	}

	c.mu.Unlock()

	for _, ID := range gameServerIDs {
		c.flushAddPlayersForServer(ID)
	}
}

func (c *client) flushRemovePlayersForServer(gameServerID uuid.UUID) {
	c.mu.Lock()
	playerIDs := c.signalBuffer.removePlayers[gameServerID]
	delete(c.signalBuffer.removePlayers, gameServerID)
	c.mu.Unlock()

	if len(playerIDs) == 0 {
		return
	}

	err := c.temporal.SignalWorkflow(
		c.ctx,
		gameServerID.String(),
		"",
		workflow.SignalPlayersLeave,
		workflow.PlayersLeaveSignal{
			PlayerIDs: playerIDs,
		},
	)
	if err != nil {
		logger := log.With().Err(err).
			Str("server_id", gameServerID.String()).
			Int("num_players", len(playerIDs)).
			Any("player_ids", playerIDs).
			Logger()

		if !c.isRetryableError(err) {
			logger.Warn().Msg("dropping removePlayers signal")
			return
		}

		logger.Error().Msg("failed to flush removePlayers signal buffer")

		c.mu.Lock()
		c.signalBuffer.removePlayers[gameServerID] = append(c.signalBuffer.removePlayers[gameServerID], playerIDs...)
		c.mu.Unlock()
	}
}

func (c *client) flushRemovePlayers() {
	c.mu.Lock()

	gameServerIDs := make([]uuid.UUID, 0, len(c.signalBuffer.removePlayers))
	for gameServerID := range c.signalBuffer.removePlayers {
		gameServerIDs = append(gameServerIDs, gameServerID)
	}

	c.mu.Unlock()

	for _, ID := range gameServerIDs {
		c.flushRemovePlayersForServer(ID)
	}
}

func (c *client) flushReportPlayersForServer(gameServerID uuid.UUID) {
	c.mu.Lock()
	reports := c.signalBuffer.reportPlayers[gameServerID]
	delete(c.signalBuffer.reportPlayers, gameServerID)
	c.mu.Unlock()

	if len(reports) == 0 {
		return
	}

	err := c.temporal.SignalWorkflow(
		c.ctx,
		gameServerID.String(),
		"",
		workflow.SignalPlayersReported,
		workflow.PlayersReportedSignal{
			Reports: reports,
		},
	)
	if err != nil {
		logger := log.With().Err(err).
			Str("server_id", gameServerID.String()).
			Any("reports", reports).
			Logger()

		if !c.isRetryableError(err) {
			logger.Warn().Msg("dropping reportPlayers signal")
			return
		}

		logger.Error().Msg("failed to flush reportPlayers signal buffer")

		c.mu.Lock()
		c.signalBuffer.reportPlayers[gameServerID] = append(c.signalBuffer.reportPlayers[gameServerID], reports...)
		c.mu.Unlock()
	}
}

func (c *client) flushReportPlayers() {
	c.mu.Lock()

	gameServerIDs := make([]uuid.UUID, 0, len(c.signalBuffer.reportPlayers))
	for gameServerID := range c.signalBuffer.reportPlayers {
		gameServerIDs = append(gameServerIDs, gameServerID)
	}

	c.mu.Unlock()

	for _, ID := range gameServerIDs {
		c.flushReportPlayersForServer(ID)
	}
}

func (c *client) flushClearPlayerReportsForServer(gameServerID uuid.UUID) {
	c.mu.Lock()
	playerIDs := c.signalBuffer.clearPlayerReports[gameServerID]
	delete(c.signalBuffer.clearPlayerReports, gameServerID)
	c.mu.Unlock()

	if len(playerIDs) == 0 {
		return
	}

	err := c.temporal.SignalWorkflow(
		c.ctx,
		gameServerID.String(),
		"",
		workflow.SignalClearPlayerReports,
		workflow.ClearPlayerReportsSignal{
			PlayerIDs: playerIDs,
		},
	)
	if err != nil {
		logger := log.With().Err(err).
			Str("server_id", gameServerID.String()).
			Any("player_ids", playerIDs).
			Logger()

		if !c.isRetryableError(err) {
			logger.Warn().Msg("dropping clearPlayerReports signal")
			return
		}

		logger.Error().Msg("failed to flush clearPlayerReports signal buffer")

		c.mu.Lock()
		c.signalBuffer.clearPlayerReports[gameServerID] = append(c.signalBuffer.clearPlayerReports[gameServerID], playerIDs...)
		c.mu.Unlock()
	}
}

func (c *client) flushClearPlayerReports() {
	c.mu.Lock()

	gameServerIDs := make([]uuid.UUID, 0, len(c.signalBuffer.clearPlayerReports))
	for gameServerID := range c.signalBuffer.clearPlayerReports {
		gameServerIDs = append(gameServerIDs, gameServerID)
	}

	c.mu.Unlock()

	for _, ID := range gameServerIDs {
		c.flushClearPlayerReportsForServer(ID)
	}
}

func (c *client) flushEjectPlayersForServer(gameServerID uuid.UUID) {
	c.mu.Lock()
	ejections := c.signalBuffer.ejectPlayers[gameServerID]
	delete(c.signalBuffer.ejectPlayers, gameServerID)
	c.mu.Unlock()

	if len(ejections) == 0 {
		return
	}

	err := c.temporal.SignalWorkflow(
		c.ctx,
		gameServerID.String(),
		"",
		workflow.SignalPlayersEjected,
		workflow.PlayersEjectedSignal{
			Ejections: ejections,
		},
	)
	if err != nil {
		logger := log.With().Err(err).
			Str("server_id", gameServerID.String()).
			Any("ejections", ejections).
			Logger()

		if !c.isRetryableError(err) {
			logger.Warn().Msg("dropping ejectPlayers signal")
			return
		}

		logger.Error().Msg("failed to flush ejectPlayers signal buffer")

		c.mu.Lock()
		c.signalBuffer.ejectPlayers[gameServerID] = append(c.signalBuffer.ejectPlayers[gameServerID], ejections...)
		c.mu.Unlock()
	}
}

func (c *client) flushEjectPlayers() {
	c.mu.Lock()

	gameServerIDs := make([]uuid.UUID, 0, len(c.signalBuffer.ejectPlayers))
	for gameServerID := range c.signalBuffer.ejectPlayers {
		gameServerIDs = append(gameServerIDs, gameServerID)
	}

	c.mu.Unlock()

	for _, ID := range gameServerIDs {
		c.flushEjectPlayersForServer(ID)
	}
}

func (c *client) flushRecordCorrectGuessesForServer(gameServerID uuid.UUID) {
	c.mu.Lock()
	gameIDGuesses := c.signalBuffer.recordCorrectGuesses[gameServerID]
	delete(c.signalBuffer.recordCorrectGuesses, gameServerID)
	c.mu.Unlock()

	for gameID, gameRoundGuesses := range gameIDGuesses {
		for gameRound, guesses := range gameRoundGuesses {
			if len(guesses) == 0 {
				continue
			}

			err := c.temporal.SignalWorkflow(
				c.ctx,
				gameServerID.String(),
				"",
				workflow.SignalCorrectGuesses,
				workflow.CorrectGuessesSignal{
					GameID:       gameID,
					CurrentRound: gameRound,
					Guesses:      guesses,
				},
			)
			if err != nil {
				logger := log.With().Err(err).
					Str("server_id", gameServerID.String()).
					Str("game_id", gameID.String()).
					Uint8("game_round", gameRound).
					Int("num_guesses", len(guesses)).
					Logger()

				if !c.isRetryableError(err) {
					logger.Warn().Msg("dropping recordCorrectGuesses signal")
					continue
				}

				logger.Error().Msg("failed to flush recordCorrectGuesses signal buffer")

				c.mu.Lock()
				slot := ensureRecordCorrectGuessesSlot(c.signalBuffer.recordCorrectGuesses, gameServerID, gameID)
				slot[gameRound] = append(slot[gameRound], guesses...)
				c.mu.Unlock()
			}
		}
	}
}

func (c *client) flushRecordCorrectGuesses() {
	c.mu.Lock()

	gameServerIDs := make([]uuid.UUID, 0, len(c.signalBuffer.recordCorrectGuesses))
	for gameServerID := range c.signalBuffer.recordCorrectGuesses {
		gameServerIDs = append(gameServerIDs, gameServerID)
	}

	c.mu.Unlock()

	for _, ID := range gameServerIDs {
		c.flushRecordCorrectGuessesForServer(ID)
	}
}

func ensureRecordCorrectGuessesSlot(
	m map[uuid.UUID]map[uuid.UUID]map[uint8][]types.CorrectGuess,
	gameServerID,
	gameID uuid.UUID,
) map[uint8][]types.CorrectGuess {
	if m[gameServerID] == nil {
		m[gameServerID] = make(map[uuid.UUID]map[uint8][]types.CorrectGuess)
	}

	if m[gameServerID][gameID] == nil {
		m[gameServerID][gameID] = make(map[uint8][]types.CorrectGuess)
	}

	return m[gameServerID][gameID]
}

func (c *client) isRetryableError(err error) bool {
	var notFound *serviceerror.NotFound
	var cancelled *serviceerror.Canceled
	var invalidArgument *serviceerror.InvalidArgument

	switch {
	case errors.As(err, &notFound):
		return false
	case errors.As(err, &cancelled):
		return false
	case errors.As(err, &invalidArgument):
		return false
	}

	return true
}

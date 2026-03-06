package gameserver

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	commonCache "github.com/spazzle-io/spazzle-api/libs/common/cache"
	db "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/sqlc"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameevents"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow/types"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/util"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/wordstore"

	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/eventbus"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	ShutdownGracePeriod     = time.Second * 30
	BroadcastTimeout        = time.Second * 5
	SendDirectMsgTimeout    = time.Second * 5
	ClientRegisterTimeout   = time.Second * 5
	ClientUnregisterTimeout = time.Second * 5
)

var ErrClosedGameServer = errors.New("game server is closed")

type Config struct {
	Env       util.Config
	Store     db.Store
	Cache     commonCache.Cache
	Bus       eventbus.EventBus
	GfClient  gameflow.Client
	WordStore wordstore.Store
}

type GameServer struct {
	Config

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	mu     sync.RWMutex

	serverID   uuid.UUID
	instanceID uuid.UUID
	gameID     uuid.UUID

	busSession eventbus.Session

	register   chan *Client
	unregister chan *Client
	broadcast  chan *OutgoingMessage
	directMsg  chan *DirectMsgPayload

	clients     map[uuid.UUID]map[uuid.UUID]*Client
	clientCount atomic.Int32
	connCount   atomic.Int32

	isClosed     atomic.Bool
	isGameActive atomic.Bool

	shutdownMu    sync.Mutex
	shutdownTimer *time.Timer

	baseLogger zerolog.Logger
	gameLogger zerolog.Logger

	currentRound    uint8
	currentArtist   uuid.UUID
	currentWord     string
	activePlayers   map[uuid.UUID]bool
	correctGuessers map[uuid.UUID]bool
}

type gameServerOptions struct {
	disableBackgroundWorkers bool
}

type Option func(options *gameServerOptions)

func WithoutBackgroundWorkers() Option {
	return func(o *gameServerOptions) {
		o.disableBackgroundWorkers = true
	}
}

func NewGameServer(
	serverID uuid.UUID,
	cfg *Config,
	opts ...Option,
) (*GameServer, error) {
	options := &gameServerOptions{}
	for _, opt := range opts {
		opt(options)
	}

	ctx, cancel := context.WithCancel(context.Background())

	gameServer := &GameServer{
		Config: *cfg,

		ctx:    ctx,
		cancel: cancel,

		serverID:   serverID,
		instanceID: uuid.New(),

		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan *OutgoingMessage),
		directMsg:  make(chan *DirectMsgPayload),

		clients: make(map[uuid.UUID]map[uuid.UUID]*Client),

		activePlayers:   make(map[uuid.UUID]bool),
		correctGuessers: make(map[uuid.UUID]bool),
	}

	gameServer.baseLogger = log.With().
		Str("server_id", gameServer.serverID.String()).
		Str("instance_id", gameServer.instanceID.String()).
		Logger()
	gameServer.gameLogger = gameServer.baseLogger

	if !options.disableBackgroundWorkers {
		gameServer.wg.Add(2)
		go gameServer.run()
		go gameServer.heartbeat()
	}

	logger := gameServer.logger()
	logger.Info().Msg("created ws game server")

	return gameServer, nil
}

func (gs *GameServer) logger() zerolog.Logger {
	return gs.gameLogger
}

func (gs *GameServer) loggerWithClient(client *Client) zerolog.Logger {
	if client == nil {
		return gs.gameLogger
	}

	return gs.gameLogger.With().
		Str("user_id", client.userID.String()).
		Str("conn_id", client.connID.String()).
		Logger()
}

func (gs *GameServer) getGameID() uuid.UUID {
	gs.mu.RLock()
	defer gs.mu.RUnlock()

	return gs.gameID
}

func (gs *GameServer) getBusSession() eventbus.Session {
	gs.mu.RLock()
	defer gs.mu.RUnlock()

	return gs.busSession
}

func (gs *GameServer) getCurrentRound() uint8 {
	gs.mu.RLock()
	defer gs.mu.RUnlock()

	return gs.currentRound
}

func (gs *GameServer) getCurrentArtist() uuid.UUID {
	gs.mu.RLock()
	defer gs.mu.RUnlock()

	return gs.currentArtist
}

func (gs *GameServer) getCurrentWord() string {
	gs.mu.RLock()
	defer gs.mu.RUnlock()

	return gs.currentWord
}

func (gs *GameServer) isActivePlayer(playerID uuid.UUID) bool {
	gs.mu.RLock()
	defer gs.mu.RUnlock()

	_, ok := gs.activePlayers[playerID]
	return ok
}

func (gs *GameServer) initializeGame() error {
	if gs.isGameActive.Load() {
		return nil
	}

	gs.mu.Lock()
	defer gs.mu.Unlock()

	server, err := gs.Store.GetServerById(gs.ctx, gs.serverID)
	if err != nil {
		return fmt.Errorf("failed to get server by id: %w", err)
	}

	stakePerGame, err := db.ParseDBNumericWeiToStr(server.StakePerGame)
	if err != nil {
		return fmt.Errorf("failed to parse stake per game: %w", err)
	}

	gameID, err := gs.GfClient.Game(gs.serverID, types.GameInput{
		GameID:          uuid.New(),
		NumRounds:       server.NumRoundsPerGame,
		DrawingDuration: time.Duration(server.RoundDurationSecs) * time.Second,
		StakePerGame:    stakePerGame,
	})
	if err != nil {
		return fmt.Errorf("failed to get or create game: %w", err)
	}

	gs.gameID = gameID
	gs.gameLogger = gs.baseLogger.With().Str("game_id", gameID.String()).Logger()

	err = gs.GfClient.HeartbeatGameServerInstance(gs.serverID, gameID, gs.instanceID)
	if err != nil {
		return fmt.Errorf("failed to register game server instance: %w", err)
	}

	gameState, err := gs.GfClient.GetGameState(gs.serverID)
	if err != nil {
		return fmt.Errorf("failed to get game state: %w", err)
	}

	gs.currentRound = gameState.CurrentRound
	gs.currentArtist = gameState.CurrentArtist
	gs.currentWord = gameState.CurrentWord.Text
	gs.activePlayers = gameState.Players
	gs.correctGuessers = make(map[uuid.UUID]bool)

	busSession, err := gs.Bus.Session(eventbus.GameIdentifier{
		GameID:       gameID,
		GameServerID: gs.serverID,
	})
	if err != nil {
		return fmt.Errorf("failed to create event bus session: %w", err)
	}

	err = busSession.Subscribe(gs.ctx, eventbus.GameEventsStreamType, eventbus.StartFromNow(), gs.handleEventBusMessage)
	if err != nil {
		return fmt.Errorf("failed to subscribe to game events stream: %w", err)
	}

	err = busSession.Subscribe(gs.ctx, eventbus.DrawingUpdatesStreamType, eventbus.StartFromNow(), gs.handleEventBusMessage)
	if err != nil {
		return fmt.Errorf("failed to subscribe to drawing updates stream: %w", err)
	}

	if gs.busSession != nil {
		gs.busSession.Close()
		gs.busSession = nil
	}
	gs.busSession = busSession

	gs.isGameActive.Store(true)

	logger := gs.logger()
	logger.Info().Msg("game initialized successfully")

	return nil
}

func (gs *GameServer) run() {
	defer gs.wg.Done()

	for {
		select {
		case c := <-gs.register:
			gs.addClient(c)
		case c := <-gs.unregister:
			gs.removeClient(c)
		case msg := <-gs.broadcast:
			gs.dispatchMsg(msg)
		case msgPayload := <-gs.directMsg:
			gs.dispatchDirectMsg(msgPayload)
		case <-gs.ctx.Done():
			gs.shutdown()
			return
		}
	}
}

func (gs *GameServer) heartbeat() {
	defer gs.wg.Done()

	logger := gs.logger()

	ticker := time.NewTicker(types.GameServerHeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if !gs.isGameActive.Load() {
				continue
			}

			err := gs.GfClient.HeartbeatGameServerInstance(gs.serverID, gs.getGameID(), gs.instanceID)
			if err != nil {
				logger.Warn().Err(err).Msg("failed to send game server instance heartbeat")
				gs.scheduleShutdown()
			}

		case <-gs.ctx.Done():
			return
		}
	}
}

func (gs *GameServer) addClient(c *Client) bool {
	err := gs.initializeGame()
	if err != nil {
		logger := gs.loggerWithClient(c)
		logger.Warn().Err(err).Msg("failed to initialize game")

		reason := JoinErrorReasonUnknown
		if errors.Is(err, gameflow.ErrGameEnding) {
			reason = JoinErrorReasonGameEnding
		}

		gs.sendError(c, ErrCodeJoinError, string(reason))
		return false
	}

	logger := gs.loggerWithClient(c)

	gs.mu.Lock()

	if _, exists := gs.clients[c.userID]; !exists {
		gs.clients[c.userID] = make(map[uuid.UUID]*Client)
		gs.clientCount.Add(1)
	}

	if _, exists := gs.clients[c.userID][c.connID]; !exists {
		gs.clients[c.userID][c.connID] = c
		gs.connCount.Add(1)
	}

	gs.mu.Unlock()

	if !c.isSpectating {
		gs.GfClient.AddPlayers(gs.serverID, gs.gameID, []uuid.UUID{c.userID})
	}

	if !gs.cancelScheduledShutdown() {
		return false
	}

	gs.sendConnectionInfoMsg(c)

	logger.Info().Msg("added client to ws game server")
	return true
}

func (gs *GameServer) removeClient(c *Client) {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	logger := gs.loggerWithClient(c)

	if _, ok := gs.clients[c.userID]; !ok {
		logger.Warn().Msg("user is not registered to ws game server")
		return
	}

	if _, ok := gs.clients[c.userID][c.connID]; !ok {
		logger.Warn().Msg("connection ID is not registered to ws game server client")
		return
	}

	delete(gs.clients[c.userID], c.connID)
	close(c.send)
	gs.connCount.Add(-1)

	if len(gs.clients[c.userID]) == 0 {
		delete(gs.clients, c.userID)
		gs.clientCount.Add(-1)
	}

	logger.Info().Msg("removed client from ws game server")

	// If there are no clients in the game server, schedule server shutdown
	if gs.clientCount.Load() == 0 {
		gs.scheduleShutdown()
	}

	if c.isSpectating || !gs.isGameActive.Load() {
		return
	}

	if c.userID == gs.currentArtist {
		err := gs.GfClient.ArtistDisconnected(gs.serverID, gs.gameID, gs.currentRound, gs.currentArtist)
		if err != nil {
			logger.Error().Err(err).Msg("failed to notify workflow that artist is disconnected")
		}
	} else {
		gs.GfClient.RemovePlayers(gs.serverID, []uuid.UUID{c.userID})
	}
}

func (gs *GameServer) removeAllClients() {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	logger := gs.logger()

	if gs.clientCount.Load() == 0 {
		logger.Debug().Msg("no clients to remove from ws game server")
		return
	}

	if gs.isGameActive.Load() {
		var nonSpectatingClients []uuid.UUID
		for userID, conns := range gs.clients {
			for _, client := range conns {
				if !client.isSpectating {
					nonSpectatingClients = append(nonSpectatingClients, userID)
					break
				}
			}
		}
		gs.GfClient.RemovePlayers(gs.serverID, nonSpectatingClients)
	}

	for userID, conns := range gs.clients {
		for connID, client := range conns {
			delete(conns, connID)
			close(client.send)
		}
		delete(gs.clients, userID)
	}

	gs.connCount.Store(0)
	gs.clientCount.Store(0)
	gs.scheduleShutdown()

	logger.Info().Msg("removed all clients from ws game server")
}

func (gs *GameServer) dispatchMsg(msg *OutgoingMessage) {
	gs.mu.RLock()
	defer gs.mu.RUnlock()

	for _, conns := range gs.clients {
		for _, client := range conns {
			logger := gs.loggerWithClient(client)

			select {
			case client.send <- msg:
			default:
				logger.Warn().Msg("could not send message to client ws send channel")

				// If client send buffer is full, drop the client connection
				if !gs.IsClosed() {
					go func(c *Client) { gs.unregister <- c }(client)
				}
			}
		}
	}
}

func (gs *GameServer) dispatchDirectMsg(directMsgPayload *DirectMsgPayload) {
	gs.mu.RLock()
	defer gs.mu.RUnlock()

	for _, recipient := range directMsgPayload.Recipients {
		if clients, ok := gs.clients[recipient.UserID]; ok {
			for connID, client := range clients {
				logger := gs.loggerWithClient(client)

				if len(recipient.ConnIDs) > 0 && !slices.Contains(recipient.ConnIDs, connID) {
					continue
				}

				if recipient.ExcludeSpectators && client.isSpectating {
					continue
				}

				select {
				case client.send <- directMsgPayload.Msg:
				default:
					logger.Warn().Msg("could not send message to client ws send channel")

					// If client send buffer is full, drop the client connection
					if !gs.IsClosed() {
						go func(c *Client) { gs.unregister <- c }(client)
					}

					if directMsgPayload.Msg.RequiresWorkflowAck {
						ackReason := "failed to write to client ws. send channel is full"
						gs.ackGameEvent(directMsgPayload.Msg.CorrelationID, gameevents.AckStatusFailed, ackReason)
					}
				}
			}
		}
	}
}

func (gs *GameServer) scheduleShutdown() {
	gs.shutdownMu.Lock()
	defer gs.shutdownMu.Unlock()

	if gs.IsClosed() {
		return
	}

	if gs.shutdownTimer != nil {
		return
	}

	gs.shutdownTimer = time.AfterFunc(ShutdownGracePeriod, func() {
		gs.shutdown()
		gs.wg.Wait()
	})

	logger := gs.logger()
	logger.Info().Msg("scheduled ws game server shutdown")
}

func (gs *GameServer) shutdown() {
	logger := gs.logger()

	isInitialServerShutdown := gs.isClosed.CompareAndSwap(false, true)
	if !isInitialServerShutdown {
		return
	}

	gs.removeAllClients()

	if gs.isGameActive.Load() {
		err := gs.GfClient.UnregisterGameServerInstance(gs.serverID, gs.instanceID)
		if err != nil {
			logger.Error().Err(err).Msg("failed to unregister game server instance from workflow")
		}

		gs.GfClient.ShutdownGameServer(gs.serverID)
	}

	if gs.getBusSession() != nil {
		gs.getBusSession().Close()
	}

	gs.cancel()

	logger.Info().Msg("ws game server shut down")
}

func (gs *GameServer) cancelScheduledShutdown() bool {
	gs.shutdownMu.Lock()
	defer gs.shutdownMu.Unlock()

	logger := gs.logger()

	if gs.shutdownTimer == nil {
		return true
	}

	if gs.shutdownTimer.Stop() {
		gs.shutdownTimer = nil
		gs.isClosed.Store(false)

		logger.Info().Msg("cancelled ws game server shutdown")
		return true
	}

	logger.Warn().Msg("could not cancel ws game server shutdown")
	return false
}

func (gs *GameServer) ackGameEvent(correlationID uuid.UUID, status gameevents.AckStatus, reason string) {
	logger := gs.logger()

	err := gs.GfClient.AcknowledgeGameEvent(gs.serverID, gameevents.EventAckPayload{
		CorrelationID: correlationID,
		InstanceID:    gs.instanceID,
		Status:        status,
		Reason:        reason,
	})
	if err != nil {
		logger.Warn().Err(err).
			Str("status", string(status)).
			Str("reason", reason).
			Msg("failed to acknowledge game event")
	}
}

func (gs *GameServer) IsClosed() bool {
	return gs.isClosed.Load()
}

func (gs *GameServer) GetServerId() uuid.UUID {
	return gs.serverID
}

func (gs *GameServer) Broadcast(msg *WsMessage) error {
	if gs.IsClosed() {
		return ErrClosedGameServer
	}

	ctx, cancel := context.WithTimeout(context.Background(), BroadcastTimeout)
	defer cancel()

	outgoingMsg := &OutgoingMessage{
		WsMessage: *msg,
	}

	select {
	case gs.broadcast <- outgoingMsg:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (gs *GameServer) SendDirectMsg(directMsgPayload *DirectMsgPayload) error {
	if gs.IsClosed() {
		return ErrClosedGameServer
	}

	ctx, cancel := context.WithTimeout(context.Background(), SendDirectMsgTimeout)
	defer cancel()

	select {
	case gs.directMsg <- directMsgPayload:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (gs *GameServer) Register(c *Client) error {
	if gs.IsClosed() {
		return ErrClosedGameServer
	}

	ctx, cancel := context.WithTimeout(context.Background(), ClientRegisterTimeout)
	defer cancel()

	select {
	case gs.register <- c:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (gs *GameServer) Unregister(c *Client) error {
	if gs.IsClosed() {
		return ErrClosedGameServer
	}

	ctx, cancel := context.WithTimeout(context.Background(), ClientUnregisterTimeout)
	defer cancel()

	select {
	case gs.unregister <- c:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

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

	isClosed atomic.Bool

	shutdownMu    sync.Mutex
	shutdownTimer *time.Timer

	currentArtist       uuid.UUID
	currentWord         string
	workflowSelectionID uuid.UUID
}

// NewGameServerOptions defines optional behaviors for the GameServer constructor.
type NewGameServerOptions struct {
	// When true (default), the GameServer's main loop is started automatically.
	// This configuration is primarily intended for unit testing.
	StartServer bool
}

func NewGameServer(
	serverID uuid.UUID,
	cfg *Config,
	opts *NewGameServerOptions,
) (*GameServer, error) {
	if opts == nil {
		opts = &NewGameServerOptions{
			StartServer: true,
		}
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
	}

	err := gameServer.initializeGame()
	if err != nil {
		return nil, err
	}

	if opts.StartServer {
		gameServer.wg.Add(2)
		go gameServer.run()
		go gameServer.heartbeat()
	}

	gameServer.getLogger(gameServer.getGameID(), nil).Info().Msg("created ws game server")

	return gameServer, nil
}

func (gs *GameServer) getLogger(gameID uuid.UUID, client *Client) *zerolog.Logger {
	logger := log.With().
		Str("server_id", gs.serverID.String()).
		Str("game_id", gameID.String()).
		Str("instance_id", gs.instanceID.String()).
		Logger()

	if client != nil {
		logger = logger.With().
			Str("user_id", client.userID.String()).
			Str("conn_id", client.connID.String()).
			Logger()
	}

	return &logger
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

func (gs *GameServer) getWorkflowSelectionID() uuid.UUID {
	gs.mu.RLock()
	defer gs.mu.RUnlock()

	return gs.workflowSelectionID
}

func (gs *GameServer) initializeGame() error {
	server, err := gs.Store.GetServerById(gs.ctx, gs.serverID)
	if err != nil {
		return fmt.Errorf("failed to get server by id: %w", err)
	}

	gameID, err := gs.GfClient.Game(gs.serverID, types.GameInput{
		NumDrawingOptions: server.NumDrawingOptions,
	})
	if err != nil {
		return fmt.Errorf("failed to get or create game: %w", err)
	}

	busSession, err := gs.Bus.Session(eventbus.GameIdentifier{
		GameID:       gameID,
		GameServerID: gs.serverID,
	})
	if err != nil {
		return fmt.Errorf("failed to create event bus session: %w", err)
	}

	// TODO: If initializing game from an end game bus message, begin streams from game marker

	err = busSession.Subscribe(gs.ctx, eventbus.GameEventsStreamType, eventbus.StartFromNow(), gs.handleEventBusMessage)
	if err != nil {
		return fmt.Errorf("failed to subscribe to game events stream: %w", err)
	}

	err = busSession.Subscribe(gs.ctx, eventbus.DrawingUpdatesStreamType, eventbus.StartFromNow(), gs.handleEventBusMessage)
	if err != nil {
		return fmt.Errorf("failed to subscribe to drawing updates stream: %w", err)
	}

	gs.mu.Lock()
	defer gs.mu.Unlock()

	if gs.busSession != nil {
		gs.busSession.Close()
		gs.busSession = nil
	}

	gs.gameID = gameID
	gs.busSession = busSession

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
		case directMsgPayload := <-gs.directMsg:
			gs.dispatchDirectMsg(directMsgPayload)
		case <-gs.ctx.Done():
			gs.shutdown()
			return
		}
	}
}

func (gs *GameServer) heartbeat() {
	defer gs.wg.Done()

	ticker := time.NewTicker(gameflow.GameServerHeartbeatInterval)
	defer ticker.Stop()

	for {
		err := gs.GfClient.HeartbeatGameServerInstance(gs.serverID, gs.instanceID)
		if err != nil {
			gs.getLogger(gs.getGameID(), nil).Error().Err(err).
				Msg("failed to send game server instance heartbeat")
		}

		select {
		case <-ticker.C:
		case <-gs.ctx.Done():
			return
		}
	}
}

func (gs *GameServer) addClient(c *Client) bool {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	if !c.isSpectating {
		gs.GfClient.AddPlayers(gs.serverID, []uuid.UUID{c.userID})
	}

	if _, ok := gs.clients[c.userID]; !ok {
		gs.clients[c.userID] = make(map[uuid.UUID]*Client)
		gs.clientCount.Add(1)
	}

	gs.clients[c.userID][c.connID] = c
	gs.connCount.Add(1)

	if !gs.cancelScheduledShutdown() {
		return false
	}

	gs.getLogger(gs.gameID, c).Info().Msg("added client to ws game server")

	return true
}

func (gs *GameServer) removeClient(c *Client) {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	if _, ok := gs.clients[c.userID]; !ok {
		gs.getLogger(gs.gameID, c).Warn().Msg("user is not registered to ws game server")
		return
	}

	if _, ok := gs.clients[c.userID][c.connID]; !ok {
		gs.getLogger(gs.gameID, c).Warn().Msg("connection ID is not registered to ws game server client")
		return
	}

	if !c.isSpectating {
		gs.GfClient.RemovePlayers(gs.serverID, []uuid.UUID{c.userID})
	}

	delete(gs.clients[c.userID], c.connID)
	close(c.send)
	gs.connCount.Add(-1)

	if len(gs.clients[c.userID]) == 0 {
		delete(gs.clients, c.userID)
		gs.clientCount.Add(-1)
	}

	gs.getLogger(gs.gameID, c).Info().Msg("removed client from ws game server")

	// If there are no clients in the game server, schedule server shutdown
	if gs.clientCount.Load() == 0 {
		gs.scheduleShutdown()
	}
}

func (gs *GameServer) removeAllClients() {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	if gs.clientCount.Load() == 0 {
		gs.getLogger(gs.gameID, nil).Debug().Msg("no clients to remove from ws game server")
		return
	}

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

	gs.getLogger(gs.gameID, nil).Info().Msg("removed all clients from ws game server")
}

func (gs *GameServer) dispatchMsg(msg *OutgoingMessage) {
	gs.mu.RLock()
	defer gs.mu.RUnlock()

	for _, conns := range gs.clients {
		for _, client := range conns {
			select {
			case client.send <- msg:
			default:
				gs.getLogger(gs.gameID, client).Warn().Msg("could not send message to client ws send channel")

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
				if len(recipient.ConnIDs) > 0 && !slices.Contains(recipient.ConnIDs, connID) {
					continue
				}

				if recipient.ExcludeSpectators && client.isSpectating {
					continue
				}

				select {
				case client.send <- directMsgPayload.Msg:
				default:
					gs.getLogger(gs.gameID, client).Warn().Msg("could not send message to client ws send channel")

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

	gs.getLogger(gs.gameID, nil).Info().Msg("scheduled ws game server shutdown")
}

func (gs *GameServer) shutdown() {
	isInitialServerShutdown := gs.isClosed.CompareAndSwap(false, true)
	if !isInitialServerShutdown {
		return
	}

	gs.removeAllClients()

	err := gs.GfClient.UnregisterGameServerInstance(gs.serverID, gs.instanceID)
	if err != nil {
		gs.getLogger(gs.getGameID(), nil).Error().Err(err).Msg("failed to unregister game server instance from workflow")
	}

	gs.GfClient.Flush()

	if gs.getBusSession() != nil {
		gs.getBusSession().Close()
	}

	gs.cancel()

	gs.getLogger(gs.getGameID(), nil).Info().Msg("ws game server shut down")
}

func (gs *GameServer) cancelScheduledShutdown() bool {
	gs.shutdownMu.Lock()
	defer gs.shutdownMu.Unlock()

	if gs.shutdownTimer == nil {
		return true
	}

	if gs.shutdownTimer.Stop() {
		gs.shutdownTimer = nil
		gs.isClosed.CompareAndSwap(true, false)

		gs.getLogger(gs.getGameID(), nil).Info().Msg("cancelled ws game server shutdown")

		return true
	}

	gs.getLogger(gs.getGameID(), nil).Warn().Msg("could not cancel ws game server shutdown")

	return false
}

func (gs *GameServer) ackGameEvent(correlationID uuid.UUID, status gameevents.AckStatus, reason string) {
	err := gs.GfClient.AcknowledgeGameEvent(gs.serverID, gameevents.EventAckPayload{
		CorrelationID: correlationID,
		InstanceID:    gs.instanceID,
		Status:        status,
		Reason:        reason,
	})
	if err != nil {
		gs.getLogger(gs.getGameID(), nil).Warn().Err(err).
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

func (gs *GameServer) GetClientConnections(userId uuid.UUID) map[uuid.UUID]*Client {
	gs.mu.RLock()
	defer gs.mu.RUnlock()

	return gs.clients[userId]
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

package gameserver

import (
	"context"
	"errors"
	"slices"
	"sync"
	"sync/atomic"
	"time"

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

// DirectMsgRecipient defines the recipient of a direct ws message.
type DirectMsgRecipient struct {
	// UserID is the target user.
	UserID uuid.UUID
	// ConnIDs, if non-empty, restricts delivery to the specified connection IDs.
	// When set, ExcludeSpectators is ignored.
	ConnIDs []uuid.UUID
	// ExcludeSpectators, if true, sends the message to only the user's non-spectating connection.
	// This filter applies even if ConnIDs is non-empty, so only listed connections
	// that are not spectating will receive the message.
	//
	// If false, and ConnIDs is empty, the message is sent to all
	// connections, including spectators.
	ExcludeSpectators bool
}

// DirectMsgPayload defines a direct ws message and its target recipients.
type DirectMsgPayload struct {
	Recipients []DirectMsgRecipient
	Msg        OutgoingMessage
}

type GameServer struct {
	serverID      uuid.UUID
	instanceID    uuid.UUID
	gameID        uuid.UUID
	bus           eventbus.EventBus
	busSession    eventbus.Session
	gfClient      gameflow.Client
	register      chan *Client
	unregister    chan *Client
	broadcast     chan []byte
	directMsg     chan *DirectMsgPayload
	clients       map[uuid.UUID]map[uuid.UUID]*Client
	clientCount   atomic.Int32
	connCount     atomic.Int32
	isClosed      atomic.Bool
	shutdownMu    sync.Mutex
	shutdownTimer *time.Timer
	mu            sync.RWMutex
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
}

// NewGameServerOptions defines optional behaviors for the GameServer constructor.
type NewGameServerOptions struct {
	// When true (default), the GameServer's main loop is started automatically.
	// This configuration is primarily intended for unit testing.
	StartServer bool
}

func NewGameServer(
	serverID uuid.UUID,
	bus eventbus.EventBus,
	gfClient gameflow.Client,
	opts *NewGameServerOptions,
) (*GameServer, error) {
	if opts == nil {
		opts = &NewGameServerOptions{
			StartServer: true,
		}
	}

	gameID, err := startGameWorkflow(gfClient, serverID)
	if err != nil {
		log.Err(err).Str("server_id", serverID.String()).Msg("failed to start game workflow")
		return nil, err
	}

	busSession, err := bus.Session(eventbus.GameIdentifier{
		GameID:       gameID,
		GameServerID: serverID,
	})
	if err != nil {
		log.
			Err(err).
			Str("server_id", serverID.String()).
			Str("game_id", gameID.String()).
			Msg("failed to create event bus session")
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	gameServer := &GameServer{
		serverID:   serverID,
		instanceID: uuid.New(),
		gameID:     gameID,
		bus:        bus,
		busSession: busSession,
		gfClient:   gfClient,
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan []byte),
		directMsg:  make(chan *DirectMsgPayload),
		clients:    make(map[uuid.UUID]map[uuid.UUID]*Client),
		ctx:        ctx,
		cancel:     cancel,
	}

	err = busSession.Subscribe(ctx, eventbus.GameEventsStreamType, eventbus.StartFromNow(), handleEventBusMessage)
	if err != nil {
		gameServer.getLogger(nil).Error().Err(err).Msg("failed to subscribe to game events stream")
		return nil, err
	}

	err = busSession.Subscribe(ctx, eventbus.DrawingUpdatesStreamType, eventbus.StartFromNow(), handleEventBusMessage)
	if err != nil {
		gameServer.getLogger(nil).Error().Err(err).Msg("failed to subscribe to drawing updates stream")
		return nil, err
	}

	if opts.StartServer {
		gameServer.wg.Add(2)
		go gameServer.run()
		go gameServer.heartbeat()
	}

	gameServer.getLogger(nil).Info().Msg("created ws game server")

	return gameServer, nil
}

func (gs *GameServer) getLogger(client *Client) *zerolog.Logger {
	logger := log.With().
		Str("server_id", gs.serverID.String()).
		Str("game_id", gs.gameID.String()).
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

func startGameWorkflow(gfClient gameflow.Client, serverID uuid.UUID) (uuid.UUID, error) {
	gameID, err := gfClient.Game(serverID, gameflow.GameInput{
		MinNumPlayersToStart: 2, // TODO: Update to use db server record
	})
	if err != nil {
		return uuid.Nil, err
	}

	return gameID, nil
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
		err := gs.gfClient.HeartbeatGameServerInstance(gs.serverID, gs.instanceID)
		if err != nil {
			gs.getLogger(nil).Error().Err(err).Msg("failed to send game server instance heartbeat")
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
		gs.gfClient.AddPlayers(gs.serverID, []uuid.UUID{c.userID})
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

	gs.getLogger(c).Info().Msg("added client to ws game server")

	return true
}

func (gs *GameServer) removeClient(c *Client) {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	if _, ok := gs.clients[c.userID]; !ok {
		gs.getLogger(c).Warn().Msg("user is not registered to ws game server")
		return
	}

	if _, ok := gs.clients[c.userID][c.connID]; !ok {
		gs.getLogger(c).Warn().Msg("connection ID is not registered to ws game server client")
		return
	}

	if !c.isSpectating {
		gs.gfClient.RemovePlayers(gs.serverID, []uuid.UUID{c.userID})
	}

	delete(gs.clients[c.userID], c.connID)
	close(c.send)
	gs.connCount.Add(-1)

	if len(gs.clients[c.userID]) == 0 {
		delete(gs.clients, c.userID)
		gs.clientCount.Add(-1)
	}

	gs.getLogger(c).Info().Msg("removed client from ws game server")

	// If there are no clients in the game server, schedule server shutdown
	if gs.clientCount.Load() == 0 {
		gs.scheduleShutdown()
	}
}

func (gs *GameServer) removeAllClients() {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	if gs.clientCount.Load() == 0 {
		gs.getLogger(nil).Debug().Msg("no clients to remove from ws game server")
		return
	}

	nonSpectatingClients := gs.nonSpectatingClients()
	gs.gfClient.RemovePlayers(gs.serverID, nonSpectatingClients)

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

	gs.getLogger(nil).Info().Msg("removed all clients from ws game server")
}

func (gs *GameServer) dispatchMsg(msg []byte) {
	gs.mu.RLock()
	defer gs.mu.RUnlock()

	outgoingMsg := OutgoingMessage{
		Data: msg,
	}

	for _, conns := range gs.clients {
		for _, client := range conns {
			select {
			case client.send <- outgoingMsg:
			default:
				// If client send buffer is full, drop the client connection
				gs.getLogger(client).Warn().Msg("could not send message to client ws send channel")
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
					// If client send buffer is full, drop the client connection
					gs.getLogger(client).Warn().Msg("could not send message to client ws send channel")
					if directMsgPayload.Msg.RequiresWorkflowAck {
						// TODO: Notify workflow that message send has failed
						_ = struct{}{}
					}
					if !gs.IsClosed() {
						go func(c *Client) { gs.unregister <- c }(client)
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

	gs.getLogger(nil).Info().Msg("scheduled ws game server shutdown")
}

func (gs *GameServer) shutdown() {
	isInitialServerShutdown := gs.isClosed.CompareAndSwap(false, true)
	if !isInitialServerShutdown {
		return
	}

	gs.removeAllClients()

	err := gs.gfClient.UnregisterGameServerInstance(gs.serverID, gs.instanceID)
	if err != nil {
		gs.getLogger(nil).Error().Err(err).Msg("failed to unregister game server instance from workflow")
	}

	gs.gfClient.Flush()

	if gs.busSession != nil {
		gs.busSession.Close()
	}

	gs.cancel()

	gs.getLogger(nil).Info().Msg("ws game server shut down")
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

		gs.getLogger(nil).Info().Msg("cancelled ws game server shutdown")

		return true
	}

	gs.getLogger(nil).Warn().Msg("could not cancel ws game server shutdown")

	return false
}

func (gs *GameServer) nonSpectatingClients() []uuid.UUID {
	activeClients := make([]uuid.UUID, 0, len(gs.clients))

	for clientID, conn := range gs.clients {
		for _, client := range conn {
			if !client.isSpectating {
				activeClients = append(activeClients, clientID)
				break
			}
		}
	}

	return activeClients
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

func (gs *GameServer) Broadcast(msg []byte) error {
	if gs.IsClosed() {
		return ErrClosedGameServer
	}

	ctx, cancel := context.WithTimeout(context.Background(), BroadcastTimeout)
	defer cancel()

	select {
	case gs.broadcast <- msg:
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

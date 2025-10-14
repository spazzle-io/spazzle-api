package websocketserver

import (
	"context"
	"errors"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	GameServerShutdownGracePeriod = time.Second * 30
	BroadcastTimeout              = time.Second * 5
	SendDirectMsgTimeout          = time.Second * 5
	ClientRegisterTimeout         = time.Second * 5
	ClientUnregisterTimeout       = time.Second * 5
)

var ErrClosedGameServer = errors.New("game server is closed")

// DirectMsgRecipient defines the recipient of a direct ws message.
// To send a message to a specific connection of a given user, add its connection ID ConnIds.
// If ConnIds is empty, the message is sent to all user ws connections.
type DirectMsgRecipient struct {
	UserId  uuid.UUID
	ConnIds []uuid.UUID
}

type DirectMsgPayload struct {
	Recipients []*DirectMsgRecipient
	Msg        []byte
}

type GameServer struct {
	serverId      uuid.UUID
	register      chan *Client
	unregister    chan *Client
	broadcast     chan []byte
	directMsg     chan *DirectMsgPayload
	clientsMu     sync.RWMutex
	clients       map[uuid.UUID]map[uuid.UUID]*Client
	clientCount   atomic.Int32
	connCount     atomic.Int32
	isClosed      atomic.Bool
	shutdownMu    sync.Mutex
	shutdownTimer *time.Timer
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
}

func NewGameServer(ctx context.Context, serverId uuid.UUID, run bool) *GameServer {
	ctx, cancel := context.WithCancel(ctx)
	gameServer := &GameServer{
		serverId:   serverId,
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan []byte),
		directMsg:  make(chan *DirectMsgPayload),
		clients:    make(map[uuid.UUID]map[uuid.UUID]*Client),
		ctx:        ctx,
		cancel:     cancel,
	}

	if run {
		gameServer.wg.Add(1)
		go gameServer.run()
	}

	gameServer.getLogger(nil).Info().Msg("created ws game server")

	return gameServer
}

func (gs *GameServer) getLogger(client *Client) *zerolog.Logger {
	logger := log.With().Str("server_id", gs.serverId.String()).Logger()

	if client != nil {
		logger = logger.With().
			Str("user_id", client.userId.String()).
			Str("conn_id", client.connId.String()).
			Logger()
	}

	return &logger
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

func (gs *GameServer) addClient(c *Client) bool {
	gs.clientsMu.Lock()
	defer gs.clientsMu.Unlock()

	if _, ok := gs.clients[c.userId]; !ok {
		gs.clients[c.userId] = make(map[uuid.UUID]*Client)
		gs.clientCount.Add(1)
	}

	gs.clients[c.userId][c.connId] = c
	gs.connCount.Add(1)

	if !gs.cancelScheduledShutdown() {
		return false
	}

	gs.getLogger(c).Info().Msg("added client to ws game server")

	return true
}

func (gs *GameServer) removeClient(c *Client) {
	gs.clientsMu.Lock()
	defer gs.clientsMu.Unlock()

	if _, ok := gs.clients[c.userId]; !ok {
		gs.getLogger(c).Warn().Msg("user is not registered to ws game server")
		return
	}

	if _, ok := gs.clients[c.userId][c.connId]; !ok {
		gs.getLogger(c).Warn().Msg("connection ID is not registered to ws game server client")
		return
	}

	delete(gs.clients[c.userId], c.connId)
	close(c.send)
	gs.connCount.Add(-1)

	if len(gs.clients[c.userId]) == 0 {
		delete(gs.clients, c.userId)
		gs.clientCount.Add(-1)
	}

	gs.getLogger(c).Info().Msg("removed client from ws game server")

	// If there are no clients in the game server, schedule server shutdown
	if gs.clientCount.Load() == 0 {
		gs.scheduleShutdown()
	}
}

func (gs *GameServer) removeAllClients() {
	gs.clientsMu.Lock()
	defer gs.clientsMu.Unlock()

	for userId, conns := range gs.clients {
		for connId, client := range conns {
			delete(conns, connId)
			close(client.send)
		}
		delete(gs.clients, userId)
	}

	gs.connCount.Store(0)
	gs.clientCount.Store(0)
	gs.scheduleShutdown()

	gs.getLogger(nil).Info().Msg("removed all clients from ws game server")
}

func (gs *GameServer) dispatchMsg(msg []byte) {
	gs.clientsMu.RLock()
	defer gs.clientsMu.RUnlock()

	for _, conns := range gs.clients {
		for _, client := range conns {
			select {
			case client.send <- msg:
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
	gs.clientsMu.RLock()
	defer gs.clientsMu.RUnlock()

	for _, recipient := range directMsgPayload.Recipients {
		if clients, ok := gs.clients[recipient.UserId]; ok {
			for connId, client := range clients {
				if len(recipient.ConnIds) > 0 && !slices.Contains(recipient.ConnIds, connId) {
					continue
				}

				select {
				case client.send <- directMsgPayload.Msg:
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

	gs.shutdownTimer = time.AfterFunc(GameServerShutdownGracePeriod, func() {
		gs.shutdown()
		gs.wg.Wait()
	})

	gs.getLogger(nil).Info().Msg("scheduled ws game server shutdown")
}

func (gs *GameServer) shutdown() {
	if gs.isClosed.CompareAndSwap(false, true) {
		gs.removeAllClients()
		gs.cancel()
		gs.getLogger(nil).Info().Msg("ws game server shut down")
	}
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

func (gs *GameServer) IsClosed() bool {
	return gs.isClosed.Load()
}

func (gs *GameServer) GetServerId() uuid.UUID {
	return gs.serverId
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

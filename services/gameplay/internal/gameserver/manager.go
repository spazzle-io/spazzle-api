package gameserver

import (
	"context"
	"sync"

	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/eventbus"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type Manager struct {
	mu          sync.RWMutex
	gameServers map[uuid.UUID]*GameServer
}

func NewManager() *Manager {
	gameServerManager := &Manager{
		gameServers: make(map[uuid.UUID]*GameServer),
	}

	gameServerManager.getLogger(uuid.Nil).Info().Msg("created game server manager")

	return gameServerManager
}

func (sm *Manager) getLogger(serverId uuid.UUID) *zerolog.Logger {
	logger := log.With().Logger()

	if serverId != uuid.Nil {
		logger = logger.With().Str("server_id", serverId.String()).Logger()
	}

	return &logger
}

func (sm *Manager) GetOrCreateGameServer(
	ctx context.Context,
	bus eventbus.EventBus,
	gfClient gameflow.Client,
	serverID uuid.UUID,
) (*GameServer, error) {
	// quick lookup of the game server on a read lock
	sm.mu.RLock()
	gameServer, ok := sm.gameServers[serverID]
	sm.mu.RUnlock()
	if ok && !gameServer.IsClosed() {
		return gameServer, nil
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()

	// lookup of the game server in case it was created before we acquired the write lock
	if gameServer, ok = sm.gameServers[serverID]; ok {
		if !gameServer.IsClosed() {
			return gameServer, nil
		}
		delete(sm.gameServers, serverID)
		sm.getLogger(serverID).Info().Msg("ws game server unregistered from game server manager")
	}

	// creating a new game server
	gameServer, err := NewGameServer(serverID, bus, gfClient, nil)
	if err != nil {
		sm.getLogger(serverID).Error().Err(err).Msg("failed to create game server")
		return nil, err
	}

	sm.gameServers[serverID] = gameServer
	sm.getLogger(serverID).Info().Msg("ws game server registered by game server manager")
	return gameServer, nil
}

func (sm *Manager) RemoveGameServerIfClosed(serverID uuid.UUID) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if gameServer, ok := sm.gameServers[serverID]; ok {
		if gameServer.IsClosed() {
			delete(sm.gameServers, serverID)
			sm.getLogger(serverID).Info().Msg("ws game server unregistered from game server manager")
		}
	}
}

func (sm *Manager) Shutdown() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	for serverID, gameServer := range sm.gameServers {
		gameServer.shutdown()
		delete(sm.gameServers, serverID)
		sm.getLogger(serverID).Info().Msg("ws game server unregistered from game server manager")
	}
}

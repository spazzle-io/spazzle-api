package gameserver

import (
	"context"
	"sync"

	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/workflow"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type Manager struct {
	mu          sync.RWMutex
	gameServers map[uuid.UUID]*GameServer
	wfClient    workflow.Client
}

func NewManager(wfClient workflow.Client) *Manager {
	gameServerManager := &Manager{
		wfClient:    wfClient,
		gameServers: make(map[uuid.UUID]*GameServer),
	}

	gameServerManager.getLogger(uuid.Nil).Info().Msg("created new game server manager")

	return gameServerManager
}

func (sm *Manager) getLogger(serverId uuid.UUID) *zerolog.Logger {
	logger := log.With().Logger()

	if serverId != uuid.Nil {
		logger = logger.With().Str("serverId", serverId.String()).Logger()
	}

	return &logger
}

func (sm *Manager) GetOrCreateGameServer(ctx context.Context, serverId uuid.UUID) *GameServer {
	// quick lookup of the game server on a read lock
	sm.mu.RLock()
	gameServer, ok := sm.gameServers[serverId]
	sm.mu.RUnlock()
	if ok && !gameServer.IsClosed() {
		return gameServer
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()

	// lookup of the game server in case it was created before we acquired the write lock
	if gameServer, ok = sm.gameServers[serverId]; ok {
		if !gameServer.IsClosed() {
			return gameServer
		}
		delete(sm.gameServers, serverId)
		sm.getLogger(serverId).Info().Msg("ws game server unregistered from game server manager")
	}

	// creating a new game server
	gameServer = NewGameServer(ctx, serverId, sm.wfClient, nil)
	sm.gameServers[serverId] = gameServer
	sm.getLogger(serverId).Info().Msg("ws game server registered by game server manager")
	return gameServer
}

func (sm *Manager) RemoveGameServerIfClosed(serverId uuid.UUID) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if gameServer, ok := sm.gameServers[serverId]; ok {
		if gameServer.IsClosed() {
			delete(sm.gameServers, serverId)
			sm.getLogger(serverId).Info().Msg("ws game server unregistered from game server manager")
		}
	}
}

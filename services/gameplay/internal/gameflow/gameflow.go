package gameflow

import (
	"github.com/google/uuid"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameevents"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow/types"
)

const GameServerHeartbeatInterval = types.GameServerHeartbeatInterval

type Client interface {
	Game(gameServerID uuid.UUID, input types.GameInput) (gameID uuid.UUID, err error)
	GetGameState(gameServerID uuid.UUID) (state *types.GameStateView, err error)
	AcknowledgeGameEvent(gameServerID uuid.UUID, payload gameevents.EventAckPayload) error

	AddPlayers(gameServerID uuid.UUID, playerIDs []uuid.UUID)
	RemovePlayers(gameServerID uuid.UUID, playerIDs []uuid.UUID)

	SelectWord(gameServerID uuid.UUID, word string, selectionID uuid.UUID) error

	HeartbeatGameServerInstance(gameServerID uuid.UUID, gameServerInstanceID uuid.UUID) error
	UnregisterGameServerInstance(gameServerID uuid.UUID, gameServerInstanceID uuid.UUID) error

	Flush()
	Close()
}

package gameflow

import (
	"github.com/google/uuid"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow/types"
)

const GameServerHeartbeatInterval = types.GameServerHeartbeatInterval

type (
	GameInput  = types.GameInput
	GameOutput = types.GameOutput
)

type Client interface {
	Game(gameServerID uuid.UUID, input GameInput) (gameID uuid.UUID, err error)

	AddPlayers(gameServerID uuid.UUID, playerIDs []uuid.UUID)
	RemovePlayers(gameServerID uuid.UUID, playerIDs []uuid.UUID)

	HeartbeatGameServerInstance(gameServerID uuid.UUID, gameServerInstanceID uuid.UUID) error
	UnregisterGameServerInstance(gameServerID uuid.UUID, gameServerInstanceID uuid.UUID) error

	Flush()
	Close()
}

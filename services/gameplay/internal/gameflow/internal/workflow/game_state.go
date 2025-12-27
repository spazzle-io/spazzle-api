package workflow

import (
	"time"

	"github.com/google/uuid"
)

const DefaultRoundNumber = 1

type GameState struct {
	Phase       PhaseEnum
	RoundNumber uint8
	StartedAt   time.Time

	Players              map[uuid.UUID]*PlayerState
	MinNumPlayersToStart uint8

	GameServerInstances             map[uuid.UUID]*GameServerInstanceState
	GameServerInstancesLastPrunedAt time.Time
	GameServerInstanceTimeout       time.Duration
}

type PhaseEnum string

const (
	PhaseWaiting      PhaseEnum = "WAITING_FOR_PLAYERS"
	PhasePrepareRound PhaseEnum = "PREPARE_ROUND"
	PhaseInRound      PhaseEnum = "IN_ROUND"
	PhaseEndRound     PhaseEnum = "END_ROUND"
	PhaseEndGame      PhaseEnum = "END_GAME"
)

type PlayerState struct {
	PlayerID    uuid.UUID
	IsConnected bool
	JoinedAt    time.Time
}

type GameServerInstanceState struct {
	InstanceID uuid.UUID
	LastSeen   time.Time
}

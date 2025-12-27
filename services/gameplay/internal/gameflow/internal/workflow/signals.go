package workflow

import "github.com/google/uuid"

const (
	SignalPlayersJoin  = "players-join"
	SignalPlayersLeave = "players-leave"

	SignalGameServerInstanceHeartbeat  = "game-server-instance-heartbeat"
	SignalGameServerUnregisterInstance = "game-server-unregister-instance"
)

type PlayersJoinSignal struct {
	PlayerIDs []uuid.UUID
}

type PlayersLeaveSignal struct {
	PlayerIDs []uuid.UUID
}

type GameServerInstanceHeartbeatSignal struct {
	InstanceID uuid.UUID
}

type GameServerInstanceUnregisterSignal struct {
	InstanceID uuid.UUID
}

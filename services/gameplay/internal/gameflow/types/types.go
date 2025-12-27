package types

import "time"

const (
	GameWorkflowTaskQueue = "game-workflow"

	GameServerInstanceTimeout   = time.Second * 15
	GameServerHeartbeatInterval = time.Second * 10
)

type GameInput struct {
	MinNumPlayersToStart uint8
}

type GameOutput struct{}

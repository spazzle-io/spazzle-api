package gameflow

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow/internal/workflow"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow/types"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/util"
	"go.temporal.io/api/enums/v1"
	temporalclient "go.temporal.io/sdk/client"
)

const (
	addPlayersInterval    = time.Second * 3
	removePlayersInterval = time.Second * 3
)

type signalBuffer struct {
	addPlayers    map[uuid.UUID][]uuid.UUID
	removePlayers map[uuid.UUID][]uuid.UUID
}

type client struct {
	temporal temporalclient.Client

	mu           sync.Mutex
	signalBuffer signalBuffer

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewClient(config util.Config) (Client, error) {
	opts := getTemporalClientOpts(config)
	tc, err := temporalclient.Dial(opts)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	c := &client{
		temporal: tc,
		signalBuffer: signalBuffer{
			addPlayers:    make(map[uuid.UUID][]uuid.UUID),
			removePlayers: make(map[uuid.UUID][]uuid.UUID),
		},
		ctx:    ctx,
		cancel: cancel,
	}

	c.wg.Add(1)
	go c.flushSignalBuffer()

	return c, nil
}

func (c *client) Game(gameServerID uuid.UUID, input types.GameInput) (gameID uuid.UUID, err error) {
	run, err := c.temporal.ExecuteWorkflow(
		c.ctx,
		temporalclient.StartWorkflowOptions{
			ID:                       gameServerID.String(),
			TaskQueue:                types.GameWorkflowTaskQueue,
			WorkflowIDConflictPolicy: enums.WORKFLOW_ID_CONFLICT_POLICY_USE_EXISTING,
		},
		workflow.GameWorkflow,
		workflow.GameWorkflowParams{
			Input: input,
		},
	)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to start game workflow: %w", err)
	}

	desc, err := c.temporal.DescribeWorkflowExecution(c.ctx, run.GetID(), "")
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to describe game workflow: %w", err)
	}

	latestRunID := desc.WorkflowExecutionInfo.Execution.RunId

	gameID, err = uuid.Parse(latestRunID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to parse game ID: %w", err)
	}

	return gameID, nil
}

func (c *client) AddPlayers(gameServerID uuid.UUID, playerIDs []uuid.UUID) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.signalBuffer.addPlayers[gameServerID] = append(c.signalBuffer.addPlayers[gameServerID], playerIDs...)
}

func (c *client) RemovePlayers(gameServerID uuid.UUID, playerIDs []uuid.UUID) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.signalBuffer.removePlayers[gameServerID] = append(c.signalBuffer.removePlayers[gameServerID], playerIDs...)
}

func (c *client) HeartbeatGameServerInstance(gameServerID uuid.UUID, gameServerInstanceID uuid.UUID) error {
	return c.temporal.SignalWorkflow(
		c.ctx,
		gameServerID.String(),
		"",
		workflow.SignalGameServerInstanceHeartbeat,
		workflow.GameServerInstanceHeartbeatSignal{
			InstanceID: gameServerInstanceID,
		},
	)
}

func (c *client) UnregisterGameServerInstance(gameServerID uuid.UUID, gameServerInstanceID uuid.UUID) error {
	return c.temporal.SignalWorkflow(
		c.ctx,
		gameServerID.String(),
		"",
		workflow.SignalGameServerUnregisterInstance,
		workflow.GameServerInstanceUnregisterSignal{
			InstanceID: gameServerInstanceID,
		},
	)
}

func (c *client) Flush() {
	c.flushAddPlayers()
	c.flushRemovePlayers()
}

func (c *client) Close() {
	c.Flush()
	c.cancel()
	c.wg.Wait()
	c.temporal.Close()
}

func (c *client) flushSignalBuffer() {
	defer c.wg.Done()

	addPlayersTicker := time.NewTicker(addPlayersInterval)
	removePlayersTicker := time.NewTicker(removePlayersInterval)

	defer addPlayersTicker.Stop()
	defer removePlayersTicker.Stop()

	for {
		select {
		case <-addPlayersTicker.C:
			c.flushAddPlayers()
		case <-removePlayersTicker.C:
			c.flushRemovePlayers()
		case <-c.ctx.Done():
			return
		}
	}
}

func (c *client) flushAddPlayers() {
	c.mu.Lock()
	pending := c.signalBuffer.addPlayers
	c.signalBuffer.addPlayers = make(map[uuid.UUID][]uuid.UUID)
	c.mu.Unlock()

	for gameServerID, playerIDs := range pending {
		if len(playerIDs) == 0 {
			continue
		}

		err := c.temporal.SignalWorkflow(
			c.ctx,
			gameServerID.String(),
			"",
			workflow.SignalPlayersJoin,
			workflow.PlayersJoinSignal{
				PlayerIDs: playerIDs,
			},
		)
		if err != nil {
			log.Error().Err(err).
				Str("server_id", gameServerID.String()).
				Any("player_ids", playerIDs).
				Msg("failed to flush addPlayers signal buffer")

			c.mu.Lock()
			c.signalBuffer.addPlayers[gameServerID] = append(c.signalBuffer.addPlayers[gameServerID], playerIDs...)
			c.mu.Unlock()
		}
	}
}

func (c *client) flushRemovePlayers() {
	c.mu.Lock()
	pending := c.signalBuffer.removePlayers
	c.signalBuffer.removePlayers = make(map[uuid.UUID][]uuid.UUID)
	c.mu.Unlock()

	for gameServerID, playerIDs := range pending {
		if len(playerIDs) == 0 {
			continue
		}

		err := c.temporal.SignalWorkflow(
			c.ctx,
			gameServerID.String(),
			"",
			workflow.SignalPlayersLeave,
			workflow.PlayersLeaveSignal{
				PlayerIDs: playerIDs,
			},
		)
		if err != nil {
			log.Error().Err(err).
				Str("server_id", gameServerID.String()).
				Any("player_ids", playerIDs).
				Msg("failed to flush removePlayers signal buffer")

			c.mu.Lock()
			c.signalBuffer.removePlayers[gameServerID] = append(c.signalBuffer.removePlayers[gameServerID], playerIDs...)
			c.mu.Unlock()
		}
	}
}

package workflow

import (
	"fmt"

	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameevents"
	"go.temporal.io/sdk/log"
	"go.temporal.io/sdk/workflow"
)

func registerGlobalSignalHandlers(ctx workflow.Context, state *GameState, notifyCh workflow.Channel, logger log.Logger) {
	workflow.Go(ctx, func(ctx workflow.Context) {
		handlePlayersJoinedSignal(ctx, state, notifyCh, logger)
	})

	workflow.Go(ctx, func(ctx workflow.Context) {
		handlePlayersLeftSignal(ctx, state, notifyCh, logger)
	})

	workflow.Go(ctx, func(ctx workflow.Context) {
		handleGameServerInstanceHeartbeatSignal(ctx, state, logger)
	})

	workflow.Go(ctx, func(ctx workflow.Context) {
		handleGameServerInstanceUnregisterSignal(ctx, state, logger)
	})

	workflow.Go(ctx, func(ctx workflow.Context) {
		handleEventAckSignal(ctx, state, notifyCh, logger)
	})
}

func handlePlayersJoinedSignal(ctx workflow.Context, state *GameState, notifyCh workflow.Channel, logger log.Logger) {
	logger.Info(fmt.Sprintf("started %s global signal handler", SignalPlayersJoin))

	ch := workflow.GetSignalChannel(ctx, SignalPlayersJoin)
	selector := workflow.NewSelector(ctx)

	selector.AddReceive(ch, func(c workflow.ReceiveChannel, more bool) {
		var sig PlayersJoinSignal
		var numPlayersJoined uint16

		c.Receive(ctx, &sig)

		for _, playerID := range sig.PlayerIDs {
			if _, exists := state.Players[playerID]; !exists {
				state.Players[playerID] = &PlayerState{
					PlayerID:    playerID,
					IsConnected: true,
					JoinedAt:    workflow.Now(ctx).UTC(),
				}
				numPlayersJoined++
			}
		}

		if numPlayersJoined > 0 {
			logger.Info(
				"players joined game workflow",
				"num_players", len(state.Players),
				"num_players_joined", numPlayersJoined,
			)
			notifyCh.Send(ctx, struct{}{})
		}
	})

	for {
		selector.Select(ctx)
	}
}

func handlePlayersLeftSignal(ctx workflow.Context, state *GameState, notifyCh workflow.Channel, logger log.Logger) {
	logger.Info(fmt.Sprintf("started %s global signal handler", SignalPlayersLeave))

	ch := workflow.GetSignalChannel(ctx, SignalPlayersLeave)
	selector := workflow.NewSelector(ctx)

	selector.AddReceive(ch, func(c workflow.ReceiveChannel, more bool) {
		var sig PlayersLeaveSignal
		var numPlayersLeft uint16

		c.Receive(ctx, &sig)

		for _, playerID := range sig.PlayerIDs {
			if _, exists := state.Players[playerID]; exists {
				delete(state.Players, playerID)
				numPlayersLeft++
			}
		}

		if numPlayersLeft > 0 {
			logger.Info(
				"players left game workflow",
				"num_players", len(state.Players),
				"num_players_left", numPlayersLeft,
			)
			notifyCh.Send(ctx, struct{}{})
		}
	})

	for {
		selector.Select(ctx)
	}
}

func handleGameServerInstanceHeartbeatSignal(ctx workflow.Context, state *GameState, logger log.Logger) {
	logger.Info(fmt.Sprintf("started %s global signal handler", SignalGameServerInstanceHeartbeat))

	ch := workflow.GetSignalChannel(ctx, SignalGameServerInstanceHeartbeat)
	selector := workflow.NewSelector(ctx)

	selector.AddReceive(ch, func(c workflow.ReceiveChannel, more bool) {
		var sig GameServerInstanceHeartbeatSignal

		c.Receive(ctx, &sig)

		state.GameServerInstances[sig.InstanceID] = &GameServerInstanceState{
			InstanceID: sig.InstanceID,
			LastSeen:   workflow.Now(ctx).UTC(),
		}

		pruneGameServerInstances(ctx, state, logger)
	})

	for {
		selector.Select(ctx)
	}
}

func handleGameServerInstanceUnregisterSignal(ctx workflow.Context, state *GameState, logger log.Logger) {
	logger.Info(fmt.Sprintf("started %s global signal handler", SignalGameServerUnregisterInstance))

	ch := workflow.GetSignalChannel(ctx, SignalGameServerUnregisterInstance)
	selector := workflow.NewSelector(ctx)

	selector.AddReceive(ch, func(c workflow.ReceiveChannel, more bool) {
		var sig GameServerInstanceUnregisterSignal

		c.Receive(ctx, &sig)

		if _, exists := state.GameServerInstances[sig.InstanceID]; exists {
			delete(state.GameServerInstances, sig.InstanceID)

			logger.Info(
				"unregistered game server instance",
				"instance_id", sig.InstanceID,
			)
		}

		pruneGameServerInstances(ctx, state, logger)
	})

	for {
		selector.Select(ctx)
	}
}

func handleEventAckSignal(ctx workflow.Context, state *GameState, notifyCh workflow.Channel, logger log.Logger) {
	logger.Info(fmt.Sprintf("started %s global signal handler", SignalEventAck))

	ch := workflow.GetSignalChannel(ctx, SignalEventAck)
	selector := workflow.NewSelector(ctx)

	selector.AddReceive(ch, func(c workflow.ReceiveChannel, more bool) {
		var sig gameevents.EventAckPayload

		c.Receive(ctx, &sig)

		pending, exists := state.PendingAcks[sig.CorrelationID]
		if !exists {
			logger.Warn(
				"received ACK for an unknown correlation ID",
				"correlation_id", sig.CorrelationID,
				"instance_id", sig.InstanceID,
			)
			return
		}

		pending.ReceivedFrom[sig.InstanceID] = sig.Status
		logger.Info(
			"received event ACK",
			"correlation_id", sig.CorrelationID,
			"instance_id", sig.InstanceID,
			"status", sig.Status,
			"received_count", len(pending.ReceivedFrom),
		)

		notifyCh.Send(ctx, struct{}{})
	})

	for {
		selector.Select(ctx)
	}
}

func pruneGameServerInstances(ctx workflow.Context, state *GameState, logger log.Logger) {
	var numPrunedInstances uint16

	currentTime := workflow.Now(ctx).UTC()
	cutoffTime := currentTime.Add(-state.GameServerInstanceTimeout)
	nextPruneTime := state.GameServerInstancesLastPrunedAt.Add(state.GameServerInstanceTimeout)

	if currentTime.Before(nextPruneTime) {
		return
	}

	for _, gameServerInstance := range state.GameServerInstances {
		if gameServerInstance.LastSeen.Before(cutoffTime) {
			delete(state.GameServerInstances, gameServerInstance.InstanceID)
			numPrunedInstances++
		}
	}

	state.GameServerInstancesLastPrunedAt = currentTime

	if numPrunedInstances > 0 {
		logger.Info(
			"pruned game server instances",
			"num_instances", len(state.GameServerInstances),
			"num_pruned_instances", numPrunedInstances,
		)
	}
}

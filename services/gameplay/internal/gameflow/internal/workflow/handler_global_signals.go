package workflow

import (
	"slices"

	commonUtil "github.com/spazzle-io/spazzle-api/libs/common/util"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameevents"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow/types"
	"go.temporal.io/sdk/workflow"
)

func registerGlobalSignalHandlers(ctx workflow.Context, state *GameState, notifyCh workflow.Channel) {
	workflow.Go(ctx, func(ctx workflow.Context) {
		handlePlayersJoinedSignal(ctx, state, notifyCh)
	})

	workflow.Go(ctx, func(ctx workflow.Context) {
		handlePlayersLeftSignal(ctx, state, notifyCh)
	})

	workflow.Go(ctx, func(ctx workflow.Context) {
		handlePlayerReports(ctx, state, notifyCh)
	})

	workflow.Go(ctx, func(ctx workflow.Context) {
		handleClearPlayerReports(ctx, state, notifyCh)
	})

	workflow.Go(ctx, func(ctx workflow.Context) {
		handlePlayerEjections(ctx, state, notifyCh)
	})

	workflow.Go(ctx, func(ctx workflow.Context) {
		handleGameServerInstanceHeartbeatSignal(ctx, state)
	})

	workflow.Go(ctx, func(ctx workflow.Context) {
		handleGameServerInstanceUnregisterSignal(ctx, state)
	})

	workflow.Go(ctx, func(ctx workflow.Context) {
		handleEventAckSignal(ctx, state, notifyCh)
	})

	workflow.Go(ctx, func(ctx workflow.Context) {
		handleTerminateGameSignal(ctx, state, notifyCh)
	})
}

func handlePlayersJoinedSignal(ctx workflow.Context, state *GameState, notifyCh workflow.Channel) {
	ch := workflow.GetSignalChannel(ctx, SignalPlayersJoin)
	selector := workflow.NewSelector(ctx)

	selector.AddReceive(ch, func(c workflow.ReceiveChannel, more bool) {
		var sig PlayersJoinSignal
		var payload gameevents.PlayersJoinedPayload

		c.Receive(ctx, &sig)

		// TODO: If endGame/endRound phase, don't accept new connections and return reason.
		// Also ensure that disconnected players still receive their won pot but ejected players don't.
		if state.Phase == types.PhaseEndGame {
			return
		}

		if sig.GameID != state.GameID {
			for _, playerID := range sig.PlayerIDs {
				payload.RejectedPlayers = append(payload.RejectedPlayers, gameevents.RejectedPlayer{
					PlayerID: playerID,
					Reason:   gameevents.RejectionReasonInvalidGame,
				})
			}
		} else {
			for _, playerID := range sig.PlayerIDs {
				if state.EjectedPlayers[playerID] {
					payload.RejectedPlayers = append(payload.RejectedPlayers, gameevents.RejectedPlayer{
						PlayerID: playerID,
						Reason:   gameevents.RejectionReasonEjectedPlayer,
					})
					continue
				}

				if player, exists := state.Players[playerID]; exists {
					player.IsConnected = true
					player.JoinedAt = workflow.Now(ctx).UTC()
				} else {
					playerIdx, err := commonUtil.IntToUint32(len(state.Players))
					if err != nil {
						state.Logger().Error("failed to compute player index",
							"num_players", len(state.Players))
						continue
					}

					state.Players[playerID] = &PlayerGameState{
						PlayerID:    playerID,
						PlayerIdx:   playerIdx,
						StakeLost:   commonUtil.ZeroWei().String(),
						IsConnected: true,
						JoinedAt:    workflow.Now(ctx).UTC(),
					}
				}

				payload.AddedPlayers = append(payload.AddedPlayers, playerID)
			}
		}

		if len(payload.AddedPlayers) > 0 || len(payload.RejectedPlayers) > 0 {
			state.Logger().Info(
				"players joined game workflow",
				"num_players_joined", len(payload.AddedPlayers),
				"num_players_rejected", len(payload.RejectedPlayers),
			)

			notifyCh.Send(ctx, struct{}{})

			_, err := sendGameEvent(ctx, state, notifyCh, gameevents.TypePlayersJoined, payload)
			if err != nil {
				state.Logger().Error("failed to send players joined event", "error", err)
			}
		}
	})

	for {
		selector.Select(ctx)
	}
}

func handlePlayersLeftSignal(ctx workflow.Context, state *GameState, notifyCh workflow.Channel) {
	ch := workflow.GetSignalChannel(ctx, SignalPlayersLeave)
	selector := workflow.NewSelector(ctx)

	selector.AddReceive(ch, func(c workflow.ReceiveChannel, more bool) {
		var sig PlayersLeaveSignal
		var payload gameevents.PlayersLeftPayload

		c.Receive(ctx, &sig)

		// TODO: Similar to player joined payload, return rejected players and reason.
		if state.Phase == types.PhaseEndGame {
			return
		}

		for _, playerID := range sig.PlayerIDs {
			if player, exists := state.Players[playerID]; exists {
				player.IsConnected = false
				player.LeftAt = workflow.Now(ctx).UTC()
				payload.PlayerIDs = append(payload.PlayerIDs, playerID)
			}
		}

		if len(payload.PlayerIDs) > 0 {
			state.Logger().Info(
				"players left game workflow",
				"num_players_left", len(payload.PlayerIDs),
			)

			notifyCh.Send(ctx, struct{}{})

			_, err := sendGameEvent(ctx, state, notifyCh, gameevents.TypePlayersLeft, payload)
			if err != nil {
				state.Logger().Error("failed to send players left event", "error", err)
			}
		}
	})

	for {
		selector.Select(ctx)
	}
}

func handlePlayerReports(ctx workflow.Context, state *GameState, notifyCh workflow.Channel) {
	ch := workflow.GetSignalChannel(ctx, SignalPlayersReported)
	selector := workflow.NewSelector(ctx)

	selector.AddReceive(ch, func(c workflow.ReceiveChannel, more bool) {
		var sig PlayersReportedSignal
		c.Receive(ctx, &sig)
		processPlayerReports(ctx, state, sig, notifyCh)
	})

	for {
		selector.Select(ctx)
	}
}

func handleClearPlayerReports(ctx workflow.Context, state *GameState, notifyCh workflow.Channel) {
	ch := workflow.GetSignalChannel(ctx, SignalClearPlayerReports)
	selector := workflow.NewSelector(ctx)

	selector.AddReceive(ch, func(c workflow.ReceiveChannel, more bool) {
		var sig ClearPlayerReportsSignal
		var payload gameevents.PlayerReportsClearedPayload

		c.Receive(ctx, &sig)

		if len(sig.PlayerIDs) == 0 {
			return
		}

		for _, playerID := range sig.PlayerIDs {
			if _, exists := state.Players[playerID]; !exists {
				continue
			}

			state.PlayerReportCounts[playerID] = 0
			payload.PlayerIDs = append(payload.PlayerIDs, playerID)
		}

		_, err := sendGameEvent(ctx, state, notifyCh, gameevents.TypePlayerReportsCleared, payload)
		if err != nil {
			state.Logger().Error("failed to send player reports cleared event", "error", err)
			return
		}

		state.Logger().Info("player reports cleared", "player_ids", sig.PlayerIDs)
	})

	for {
		selector.Select(ctx)
	}
}

func handlePlayerEjections(ctx workflow.Context, state *GameState, notifyCh workflow.Channel) {
	ch := workflow.GetSignalChannel(ctx, SignalPlayersEjected)
	selector := workflow.NewSelector(ctx)

	selector.AddReceive(ch, func(c workflow.ReceiveChannel, more bool) {
		var sig PlayersEjectedSignal
		c.Receive(ctx, &sig)

		if state.Phase == types.PhaseEndGame {
			return
		}

		var ejections []gameevents.PlayerEjection

		for _, ejection := range sig.Ejections {
			if player, exists := state.Players[ejection.PlayerID]; exists {
				player.IsEjected = true
				player.EjectedAt = workflow.Now(ctx).UTC()
				state.EjectedPlayers[ejection.PlayerID] = true

				// TODO: Fine ejected player and reset player scores to zero

				ejections = append(ejections, gameevents.PlayerEjection{
					PlayerID:     ejection.PlayerID,
					IsArtist:     state.CurrentArtist == ejection.PlayerID,
					Ejector:      ejection.Ejector,
					TotalReports: state.PlayerReportCounts[ejection.PlayerID],
				})
			}
		}

		if len(ejections) == 0 {
			return
		}

		notifyCh.Send(ctx, struct{}{})

		payload := gameevents.PlayersEjectedPayload{
			CurrentRound: state.CurrentRound,
			Ejections:    ejections,
		}
		_, err := sendGameEvent(ctx, state, notifyCh, gameevents.TypePlayersEjected, payload)
		if err != nil {
			state.Logger().Error("failed to send players ejected event", "error", err)
			return
		}

		state.Logger().Info("players ejected", "ejections", sig.Ejections)
	})

	for {
		selector.Select(ctx)
	}
}

func handleGameServerInstanceHeartbeatSignal(ctx workflow.Context, state *GameState) {
	ch := workflow.GetSignalChannel(ctx, SignalGameServerInstanceHeartbeat)
	selector := workflow.NewSelector(ctx)

	selector.AddReceive(ch, func(c workflow.ReceiveChannel, more bool) {
		var sig GameServerInstanceHeartbeatSignal

		c.Receive(ctx, &sig)

		state.GameServerInstances[sig.InstanceID] = &GameServerInstanceState{
			InstanceID: sig.InstanceID,
			LastSeen:   workflow.Now(ctx).UTC(),
		}

		pruneGameServerInstances(ctx, state)
	})

	for {
		selector.Select(ctx)
	}
}

func handleGameServerInstanceUnregisterSignal(ctx workflow.Context, state *GameState) {
	ch := workflow.GetSignalChannel(ctx, SignalGameServerInstanceUnregistered)
	selector := workflow.NewSelector(ctx)

	selector.AddReceive(ch, func(c workflow.ReceiveChannel, more bool) {
		var sig GameServerInstanceUnregisteredSignal

		c.Receive(ctx, &sig)

		if _, exists := state.GameServerInstances[sig.InstanceID]; exists {
			delete(state.GameServerInstances, sig.InstanceID)

			state.Logger().Info(
				"unregistered game server instance",
				"instance_id", sig.InstanceID,
			)
		}

		pruneGameServerInstances(ctx, state)
	})

	for {
		selector.Select(ctx)
	}
}

func handleEventAckSignal(ctx workflow.Context, state *GameState, notifyCh workflow.Channel) {
	ch := workflow.GetSignalChannel(ctx, SignalEventAck)
	selector := workflow.NewSelector(ctx)

	selector.AddReceive(ch, func(c workflow.ReceiveChannel, more bool) {
		var sig gameevents.EventAckPayload

		c.Receive(ctx, &sig)

		pending, exists := state.PendingAcks[sig.CorrelationID]
		if !exists {
			state.Logger().Warn(
				"received ACK for an unknown correlation ID",
				"correlation_id", sig.CorrelationID,
				"instance_id", sig.InstanceID,
			)
			return
		}

		pending.ReceivedFrom[sig.InstanceID] = sig.Status
		state.Logger().Info(
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

func handleTerminateGameSignal(ctx workflow.Context, state *GameState, notifyCh workflow.Channel) {
	ch := workflow.GetSignalChannel(ctx, SignalTerminateGame)
	selector := workflow.NewSelector(ctx)

	selector.AddReceive(ch, func(c workflow.ReceiveChannel, more bool) {
		var sig TerminateGameSignal
		c.Receive(ctx, &sig)

		if sig.GameID != state.GameID || state.IsTerminated {
			return
		}

		state.IsTerminated = true
		notifyCh.Send(ctx, struct{}{})

		state.Logger().Info("terminating game", "reason", sig.Reason)
	})

	for {
		selector.Select(ctx)
	}
}

func pruneGameServerInstances(ctx workflow.Context, state *GameState) {
	var numPrunedInstances uint16

	currentTime := workflow.Now(ctx).UTC()
	cutoffTime := currentTime.Add(-types.GameServerInstanceTimeout)
	nextPruneTime := state.GameServerInstancesLastPrunedAt.Add(types.GameServerInstanceTimeout)

	if currentTime.Before(nextPruneTime) {
		return
	}

	for _, instanceID := range sortedUUIDs(state.GameServerInstances) {
		instance := state.GameServerInstances[instanceID]
		if instance.LastSeen.Before(cutoffTime) {
			delete(state.GameServerInstances, instance.InstanceID)
			numPrunedInstances++
		}
	}

	state.GameServerInstancesLastPrunedAt = currentTime

	if numPrunedInstances > 0 {
		state.Logger().Info(
			"pruned game server instances",
			"num_instances", len(state.GameServerInstances),
			"num_pruned_instances", numPrunedInstances,
		)
	}
}

func processPlayerReports(
	ctx workflow.Context,
	state *GameState,
	signal PlayersReportedSignal,
	notifyCh workflow.Channel,
) {
	var reports []gameevents.PlayerReport

	for _, report := range signal.Reports {
		reportedPlayerIdx := state.Players[report.ReportTarget].PlayerIdx
		existingReporterTargets := state.PlayerReportsMade[report.Reporter]

		if reporter, exists := state.Players[report.Reporter]; !exists || !isActivePlayer(reporter) {
			state.Logger().Warn("invalid player report. invalid reporter",
				"reporter", report.Reporter,
				"is_reporter_connected", reporter.IsConnected,
				"is_reporter_ejected", reporter.IsEjected)
			continue
		}

		if reportTarget, exists := state.Players[report.ReportTarget]; !exists || !isActivePlayer(reportTarget) {
			state.Logger().Warn("invalid player report. invalid report target",
				"report_target", report.ReportTarget,
				"is_report_target_connected", reportTarget.IsConnected,
				"is_report_target_ejected", reportTarget.IsEjected)
			continue
		}

		if report.Reporter == report.ReportTarget {
			state.Logger().Warn(
				"invalid player report. self reporting is not allowed",
				"reporter", report.Reporter, "report_target", report.ReportTarget,
			)
			continue
		}

		if len(existingReporterTargets) > types.MaxPlayerReports {
			state.Logger().Warn("reporter exceeded maximum allowed reports",
				"reporter", report.Reporter, "max_allowed_reports", types.MaxPlayerReports)
			continue
		}

		if slices.Contains(existingReporterTargets, reportedPlayerIdx) {
			state.Logger().Info("duplicate player report ignored",
				"reporter", report.Reporter, "report_target", report.ReportTarget)
			continue
		}

		if state.PlayerReportsMade[report.Reporter] == nil {
			state.PlayerReportsMade[report.Reporter] = make([]uint32, 0)
		}

		state.PlayerReportCounts[report.ReportTarget]++
		state.PlayerReportsMade[report.Reporter] = append(
			state.PlayerReportsMade[report.Reporter], reportedPlayerIdx,
		)

		reports = append(reports, gameevents.PlayerReport{
			Reporter:       report.Reporter,
			ReportedPlayer: report.ReportTarget,
			IsArtist:       state.CurrentArtist == report.ReportTarget,
			TotalReports:   state.PlayerReportCounts[report.ReportTarget],
		})

		state.Logger().Info("player report recorded",
			"reporter", report.Reporter, "report_target", report.ReportTarget,
			"total_reports", state.PlayerReportCounts[report.ReportTarget])
	}

	if len(reports) == 0 {
		return
	}

	payload := gameevents.PlayersReportedPayload{
		CurrentRound: state.CurrentRound,
		Reports:      reports,
	}
	_, err := sendGameEvent(ctx, state, notifyCh, gameevents.TypePlayersReported, payload)
	if err != nil {
		state.Logger().Error("failed to send players reported event", "error", err)
	}
}

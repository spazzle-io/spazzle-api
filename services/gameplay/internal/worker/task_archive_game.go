package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	commonUtil "github.com/spazzle-io/spazzle-api/libs/common/util"
	db "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/sqlc"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameevents"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/rs/zerolog/log"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/eventbus"
)

const (
	TaskArchiveGame    = "task:archive_game"
	archiveBucket      = "game-archive"
	archiveReplayLimit = 5000
)

type PayloadArchiveGame struct {
	ServerID      uuid.UUID `json:"server_id"`
	GameID        uuid.UUID `json:"game_id"`
	GameStake     string    `json:"game_stake"`
	GameStartedAt time.Time `json:"game_started_at"`
	GameEndedAt   time.Time `json:"game_ended_at"`
}

func (distributor *RedisTaskDistributor) DistributeTaskArchiveGame(
	ctx context.Context,
	payload *PayloadArchiveGame,
	opts ...asynq.Option,
) error {
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal task payload: %w", err)
	}

	task := asynq.NewTask(TaskArchiveGame, jsonPayload, opts...)

	taskID := fmt.Sprintf("%s:%s", payload.ServerID.String(), payload.GameID.String())
	info, err := distributor.client.EnqueueContext(ctx, task, asynq.TaskID(taskID))
	if err != nil {
		if errors.Is(err, asynq.ErrTaskIDConflict) {
			log.Info().Str("task_id", taskID).Msg("archive task already enqueued, skipping")
			return nil
		}
		return fmt.Errorf("failed to enqueue task: %w", err)
	}

	log.Info().
		Str("type", task.Type()).
		Bytes("payload", task.Payload()).
		Str("queue", info.Queue).
		Int("max_retry", info.MaxRetry).
		Msg("enqueued task")

	return nil
}

func (processor *RedisTaskProcessor) ProcessTaskArchiveGame(ctx context.Context, task *asynq.Task) error {
	var payload PayloadArchiveGame
	err := json.Unmarshal(task.Payload(), &payload)
	if err != nil {
		return fmt.Errorf("failed to unmarshal task payload: %v: %w", err, asynq.SkipRetry)
	}

	game := eventbus.GameIdentifier{
		GameServerID: payload.ServerID,
		GameID:       payload.GameID,
	}

	gameEventsMessages, err := processor.archiveStreamsToS3(ctx, game, payload)
	if err != nil {
		return err
	}

	if err := processor.archiveGameToDB(ctx, payload, gameEventsMessages); err != nil {
		return err
	}

	if err := processor.bus.Cleanup(ctx, game); err != nil {
		return fmt.Errorf("failed to clean up game streams: %w", err)
	}

	log.Info().
		Str("type", task.Type()).
		Bytes("payload", task.Payload()).
		Msg("game archival task successfully processed")

	return nil
}

func (processor *RedisTaskProcessor) archiveStreamsToS3(
	ctx context.Context,
	game eventbus.GameIdentifier,
	payload PayloadArchiveGame,
) ([]eventbus.Message, error) {
	var gameEventsMessages []eventbus.Message

	for _, streamType := range eventbus.AllStreamTypes {
		messages, err := processor.replayGameStream(ctx, game, streamType)
		if err != nil {
			return nil, err
		}

		if streamType == eventbus.GameEventsStreamType {
			gameEventsMessages = messages
		}

		if len(messages) == 0 {
			continue
		}

		data, err := json.Marshal(messages)
		if err != nil {
			return nil, err
		}

		key := fmt.Sprintf("games/%s/%s/%s.json", payload.ServerID, payload.GameID, streamType)
		err = processor.objectStore.Put(ctx, archiveBucket, key, bytes.NewReader(data), "application/json")
		if err != nil {
			return nil, err
		}
	}

	return gameEventsMessages, nil
}

func (processor *RedisTaskProcessor) archiveGameToDB(
	ctx context.Context,
	payload PayloadArchiveGame,
	gameEventsMessages []eventbus.Message,
) error {
	if len(gameEventsMessages) == 0 {
		return nil
	}

	gameEndedPayload, err := extractGameEndedPayload(gameEventsMessages)
	if err != nil {
		return fmt.Errorf("failed to extract game ended payload: %v: %w", err, asynq.SkipRetry)
	}

	playerResults, err := mapPlayerResults(gameEndedPayload.Results)
	if err != nil {
		return fmt.Errorf("failed to map player results: %w", err)
	}

	totalPot, err := commonUtil.NewNonNegativeWei(gameEndedPayload.TotalPot)
	if err != nil {
		return fmt.Errorf("failed to parse total pot: %v: %w", err, asynq.SkipRetry)
	}

	gameStake, err := commonUtil.NewNonNegativeWei(payload.GameStake)
	if err != nil {
		return fmt.Errorf("failed to parse game stake: %v: %w", err, asynq.SkipRetry)
	}

	txParams := db.ArchiveGameTxParams{
		GameID:        payload.GameID,
		ServerID:      payload.ServerID,
		NumRounds:     int32(gameEndedPayload.TotalRounds),
		TotalPot:      totalPot,
		GameStake:     gameStake,
		PlayerResults: playerResults,
		StartedAt:     payload.GameStartedAt,
		EndedAt:       payload.GameEndedAt,
	}

	_, err = processor.store.ArchiveGameTx(ctx, txParams)
	if err != nil {
		if errors.Is(err, db.ErrGameAlreadyExists) {
		} else {
			return err
		}
	}

	return nil
}

func (processor *RedisTaskProcessor) replayGameStream(
	ctx context.Context,
	game eventbus.GameIdentifier,
	streamType eventbus.StreamType,
) ([]eventbus.Message, error) {
	var messages []eventbus.Message
	after := "0"

	for {
		result, err := processor.bus.Replay(ctx, uuid.Nil, game, streamType, eventbus.ReplayVisibilityAll, "0", after, archiveReplayLimit)
		if err != nil {
			return nil, fmt.Errorf("failed to replay stream %s: %w", streamType, err)
		}

		messages = append(messages, result.Messages...)

		if !result.HasMore {
			break
		}

		after = result.LastID
	}

	return messages, nil
}

func extractGameEndedPayload(gameEventsMessages []eventbus.Message) (*gameevents.GameEndedPayload, error) {
	for i := len(gameEventsMessages) - 1; i >= 0; i-- {
		if gameEventsMessages[i].Type == gameevents.TypeGameEnded {
			var payload gameevents.GameEndedPayload
			if err := json.Unmarshal(gameEventsMessages[i].Payload, &payload); err != nil {
				return nil, fmt.Errorf("failed to unmarshal game ended payload: %w", err)
			}
			return &payload, nil
		}
	}

	return nil, fmt.Errorf("game_ended event not found in stream")
}

func mapPlayerResults(results []*gameevents.PlayerFinalResult) ([]db.GamePlayerResult, error) {
	playerResults := make([]db.GamePlayerResult, 0, len(results))

	for _, r := range results {
		payout, err := commonUtil.NewNonNegativeWei(r.ProvisionalPayout)
		if err != nil {
			return nil, fmt.Errorf("failed to parse provisional payout: %v: %w", err, asynq.SkipRetry)
		}

		stakeLost, err := commonUtil.NewNonNegativeWei(r.TotalStakeLost)
		if err != nil {
			return nil, fmt.Errorf("failed to parse stake lost: %v: %w", err, asynq.SkipRetry)
		}

		pnl := payout.Sub(stakeLost)

		score, err := commonUtil.Int64ToInt32(r.TotalPoints)
		if err != nil {
			return nil, fmt.Errorf("failed to convert total points to int32: %w", err)
		}

		position, err := commonUtil.IntToInt32(r.Position)
		if err != nil {
			return nil, fmt.Errorf("failed to convert position to int32: %w", err)
		}

		playerResults = append(playerResults, db.GamePlayerResult{
			UserID:            r.PlayerID,
			Score:             score,
			Pnl:               pnl,
			Position:          position,
			RoundsPlayed:      int32(r.RoundsPlayed),
			ProvisionalPayout: payout,
			TotalStakeLost:    stakeLost,
			IsEvicted:         r.IsEjected,
		})
	}

	return playerResults, nil
}

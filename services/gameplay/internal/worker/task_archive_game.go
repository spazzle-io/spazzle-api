package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/rs/zerolog"
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

	logger := log.With().
		Str("game_server_id", payload.ServerID.String()).
		Str("game_id", payload.GameID.String()).
		Logger()

	game := eventbus.GameIdentifier{
		GameServerID: payload.ServerID,
		GameID:       payload.GameID,
	}

	gameEventsMessages, err := processor.archiveStreamsToS3(ctx, game, payload, logger)
	if err != nil {
		return err
	}

	if err := processor.archiveGameToDB(ctx, payload, gameEventsMessages, logger); err != nil {
		return err
	}

	if err := processor.bus.Cleanup(ctx, game); err != nil {
		logger.Error().Err(err).Msg("failed to clean up game streams")
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
	logger zerolog.Logger,
) ([]eventbus.Message, error) {
	var gameEventsMessages []eventbus.Message

	for _, streamType := range eventbus.AllStreamTypes {
		messages, err := processor.replayGameStream(ctx, game, streamType)
		if err != nil {
			logger.Error().Err(err).Str("stream_type", string(streamType)).Msg("failed to replay stream")
			return nil, err
		}

		if streamType == eventbus.GameEventsStreamType {
			gameEventsMessages = messages
		}

		if len(messages) == 0 {
			logger.Info().Str("stream_type", string(streamType)).Msg("no messages to archive")
			continue
		}

		data, err := json.Marshal(messages)
		if err != nil {
			logger.Error().Err(err).Str("stream_type", string(streamType)).Msg("failed to marshal messages")
			return nil, err
		}

		key := fmt.Sprintf("games/%s/%s/%s.json", payload.ServerID, payload.GameID, streamType)
		err = processor.objectStore.Put(ctx, archiveBucket, key, bytes.NewReader(data), "application/json")
		if err != nil {
			logger.Error().Err(err).Str("stream_type", string(streamType)).Msg("failed to upload archive")
			return nil, err
		}

		logger.Info().
			Str("stream_type", string(streamType)).
			Int("message_count", len(messages)).
			Msg("archived stream")
	}

	return gameEventsMessages, nil
}

func (processor *RedisTaskProcessor) archiveGameToDB(
	ctx context.Context,
	payload PayloadArchiveGame,
	gameEventsMessages []eventbus.Message,
	logger zerolog.Logger,
) error {
	if len(gameEventsMessages) == 0 {
		logger.Info().Msg("no game events to archive")
		return nil
	}

	gameEndedPayload, err := extractGameEndedPayload(gameEventsMessages)
	if err != nil {
		logger.Error().Err(err).Msg("failed to extract game ended payload")
		return fmt.Errorf("failed to extract game ended payload: %v: %w", err, asynq.SkipRetry)
	}

	playerResults, err := mapPlayerResults(gameEndedPayload.Results)
	if err != nil {
		logger.Error().Err(err).Msg("failed to map player results")
		return fmt.Errorf("failed to map player results: %w", err)
	}

	txParams := db.ArchiveGameTxParams{
		GameID:        payload.GameID,
		ServerID:      payload.ServerID,
		NumRounds:     int32(gameEndedPayload.TotalRounds),
		TotalPot:      commonUtil.ParseBigIntOrZero(gameEndedPayload.TotalPot),
		GameStake:     commonUtil.ParseBigIntOrZero(payload.GameStake),
		PlayerResults: playerResults,
		StartedAt:     payload.GameStartedAt,
		EndedAt:       payload.GameEndedAt,
	}

	_, err = processor.store.ArchiveGameTx(ctx, txParams)
	if err != nil {
		if errors.Is(err, db.ErrGameAlreadyExists) {
			logger.Info().Msg("game already archived in database, skipping")
		} else {
			logger.Error().Err(err).Msg("failed to archive game to database")
			return err
		}
	}

	logger.Info().Msg("archived game to database")

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
		result, err := processor.bus.Replay(ctx, uuid.Nil, game, streamType, eventbus.ReplayVisibilityAll, after, archiveReplayLimit)
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
		payout := commonUtil.ParseBigIntOrZero(r.ProvisionalPayout)
		stakeLost := commonUtil.ParseBigIntOrZero(r.TotalStakeLost)
		pnl := new(big.Int).Sub(payout, stakeLost)

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

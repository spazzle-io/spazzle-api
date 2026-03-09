package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

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
	GameServerID uuid.UUID `json:"game_server_id"`
	GameID       uuid.UUID `json:"game_id"`
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

	info, err := distributor.client.EnqueueContext(ctx, task)
	if err != nil {
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
		Str("game_server_id", payload.GameServerID.String()).
		Str("game_id", payload.GameID.String()).
		Logger()

	game := eventbus.GameIdentifier{
		GameServerID: payload.GameServerID,
		GameID:       payload.GameID,
	}

	for _, streamType := range eventbus.AllStreamTypes {
		messages, err := processor.replayGameStream(ctx, game, streamType)
		if err != nil {
			logger.Error().Err(err).Str("stream_type", string(streamType)).Msg("failed to replay stream")
			return err
		}

		if len(messages) == 0 {
			logger.Info().Str("stream_type", string(streamType)).Msg("no messages to archive")
			continue
		}

		data, err := json.Marshal(messages)
		if err != nil {
			logger.Error().Err(err).Str("stream_type", string(streamType)).Msg("failed to marshal messages")
			return err
		}

		key := fmt.Sprintf("games/%s/%s/%s.json", payload.GameServerID, payload.GameID, streamType)
		err = processor.objectStore.Put(ctx, archiveBucket, key, bytes.NewReader(data), "application/json")
		if err != nil {
			logger.Error().Err(err).Str("stream_type", string(streamType)).Msg("failed to upload archive")
			return err
		}

		logger.Info().
			Str("stream_type", string(streamType)).
			Int("message_count", len(messages)).
			Msg("archived stream")
	}

	if err := processor.bus.Cleanup(ctx, game); err != nil {
		logger.Error().Err(err).Msg("failed to clean up game streams")
	}

	log.Info().
		Str("type", task.Type()).
		Bytes("payload", task.Payload()).
		Msg("processed task")

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
		result, err := processor.bus.Replay(ctx, uuid.Nil, game, streamType, after, archiveReplayLimit)
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

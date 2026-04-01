package worker

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog/log"
)

const (
	TaskRecomputeServerTrendingScores = "task:recompute_server_trending_scores"
	trendingWindowMins                = 15
)

func (processor *RedisTaskProcessor) ProcessTaskRecomputeTrending(ctx context.Context, _ *asynq.Task) error {
	logger := log.With().
		Str("task", TaskRecomputeServerTrendingScores).
		Int("trending_window_mins", trendingWindowMins).
		Logger()

	trendingWindow := pgtype.Interval{
		Microseconds: trendingWindowMins * 60 * 1000 * 1000,
		Valid:        true,
	}

	err := processor.store.RecomputeTrendingScores(ctx, trendingWindow)
	if err != nil {
		logger.Error().Err(err).Msg("failed to recompute server trending scores")
		return fmt.Errorf("failed to recompute server trending scores: %w", err)
	}

	err = processor.store.ResetTrendingScores(ctx, trendingWindow)
	if err != nil {
		logger.Error().Err(err).Msg("failed to reset server trending scores")
		return fmt.Errorf("failed to reset server trending scores: %w", err)
	}

	logger.Info().Msg("successfully recomputed server trending scores")
	return nil
}

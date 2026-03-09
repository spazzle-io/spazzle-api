package activities

import (
	"context"
	"time"

	"github.com/hibiken/asynq"

	"github.com/google/uuid"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/worker"
)

type ArchiveGameParams struct {
	GameServerID uuid.UUID
	GameID       uuid.UUID
}

type ArchiveGameResult struct{}

func (a *Activities) ArchiveGame(
	ctx context.Context,
	params ArchiveGameParams,
) (*ArchiveGameResult, error) {
	err := a.TaskDistributor.DistributeTaskArchiveGame(
		ctx,
		&worker.PayloadArchiveGame{
			GameServerID: params.GameServerID,
			GameID:       params.GameID,
		},
		asynq.MaxRetry(5),
		asynq.Timeout(10*time.Minute),
		asynq.Retention(24*time.Hour),
	)
	return &ArchiveGameResult{}, err
}

package activities

import (
	"context"
	"time"

	"github.com/hibiken/asynq"

	"github.com/google/uuid"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/worker"
)

const (
	archiveGameMaxRetries = 5
	archiveGameTimeout    = 10 * time.Minute
)

type ArchiveGameParams struct {
	ServerID      uuid.UUID
	GameID        uuid.UUID
	GameStake     string
	GameStartedAt time.Time
	GameEndedAt   time.Time
}

type ArchiveGameResult struct{}

func (a *Activities) ArchiveGame(
	ctx context.Context,
	params ArchiveGameParams,
) (*ArchiveGameResult, error) {
	err := a.TaskDistributor.DistributeTaskArchiveGame(
		ctx,
		&worker.PayloadArchiveGame{
			ServerID:      params.ServerID,
			GameID:        params.GameID,
			GameStake:     params.GameStake,
			GameStartedAt: params.GameStartedAt,
			GameEndedAt:   params.GameEndedAt,
		},
		asynq.MaxRetry(archiveGameMaxRetries),
		asynq.Timeout(archiveGameTimeout),
	)
	return &ArchiveGameResult{}, err
}

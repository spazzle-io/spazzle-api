package activities

import (
	"context"

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
	err := a.TaskDistributor.DistributeTaskArchiveGame(ctx, &worker.PayloadArchiveGame{
		GameServerID: params.GameServerID,
		GameID:       params.GameID,
	})
	return &ArchiveGameResult{}, err
}

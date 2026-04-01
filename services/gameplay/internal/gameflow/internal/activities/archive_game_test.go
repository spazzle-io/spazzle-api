package activities

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestArchiveGame(t *testing.T) {
	deps := setupActivities(t)

	params := ArchiveGameParams{
		ServerID:      uuid.New(),
		GameID:        uuid.New(),
		GameStake:     "100",
		GameStartedAt: time.Now().UTC(),
		GameEndedAt:   time.Now().UTC(),
	}

	deps.TaskDistributor.EXPECT().
		DistributeTaskArchiveGame(gomock.Any(), gomock.Any(), gomock.Any()).
		Times(1).
		Return(nil)

	expectedResult := &ArchiveGameResult{}

	result, err := deps.Activities.ArchiveGame(context.Background(), params)
	require.NoError(t, err)
	require.Equal(t, expectedResult, result)
}

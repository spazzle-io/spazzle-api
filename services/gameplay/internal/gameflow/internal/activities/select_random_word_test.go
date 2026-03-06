package activities

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/wordstore"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestSelectRandomWord(t *testing.T) {
	deps := setupActivities(t)

	serverID := uuid.New()

	params := SelectRandomWordParams{
		GameServerID: serverID,
	}

	deps.WordStore.EXPECT().
		GetRandomWords(gomock.Any(), gomock.Eq(deps.Store), gomock.Eq(serverID), gomock.Eq(1)).
		Times(1).
		Return([]wordstore.Word{
			{
				Id:       uuid.New(),
				Word:     "random word",
				ServerID: serverID,
				AddedAt:  time.Now().UTC(),
			},
		}, nil)

	expectedResult := &SelectRandomWordResult{
		Word: "random word",
	}

	result, err := deps.Activities.SelectRandomWord(context.Background(), params)
	require.NoError(t, err)
	require.Equal(t, expectedResult, result)
}

func TestSelectRandomWord_NoAvailableWords(t *testing.T) {
	deps := setupActivities(t)

	serverID := uuid.New()

	params := SelectRandomWordParams{
		GameServerID: serverID,
	}

	deps.WordStore.EXPECT().
		GetRandomWords(gomock.Any(), gomock.Eq(deps.Store), gomock.Eq(serverID), gomock.Eq(1)).
		Times(1).
		Return([]wordstore.Word{}, nil)

	result, err := deps.Activities.SelectRandomWord(context.Background(), params)
	require.Error(t, err)
	require.Empty(t, result)
}

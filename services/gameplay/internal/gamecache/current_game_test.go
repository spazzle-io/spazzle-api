package gamecache

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	mockcache "github.com/spazzle-io/spazzle-api/libs/common/cache/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestSetCurrentGame(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cache := mockcache.NewMockCache(ctrl)

	serverID := uuid.New()
	expectedKey := fmt.Sprintf("test-current_game:%s", serverID.String())

	cache.EXPECT().
		Set(gomock.Any(), gomock.Eq(expectedKey), gomock.Any(), gomock.Eq(currentGameTTL)).
		Times(1).
		Return(nil)

	gameCache := newTestGameCache(cache)

	err := gameCache.SetCurrentGame(context.Background(), serverID, "")
	require.NoError(t, err)
}

func TestGetCurrentGame(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cache := mockcache.NewMockCache(ctrl)

	serverID := uuid.New()
	expectedKey := fmt.Sprintf("test-current_game:%s", serverID.String())

	cache.EXPECT().
		Get(gomock.Any(), gomock.Eq(expectedKey), gomock.Any()).
		Times(1).
		Return(nil)

	gameCache := newTestGameCache(cache)

	err := gameCache.GetCurrentGame(context.Background(), serverID, "")
	require.NoError(t, err)
}

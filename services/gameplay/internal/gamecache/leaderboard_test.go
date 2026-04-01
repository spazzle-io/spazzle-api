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

func TestSetLeaderboard(t *testing.T) {
	t.Run("global leaderboard", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		cache := mockcache.NewMockCache(ctrl)

		expectedKey := "test-leaderboard:global:monthly:3"

		cache.EXPECT().
			Set(gomock.Any(), gomock.Eq(expectedKey), gomock.Any(), gomock.Eq(leaderboardTTLs[LeaderboardWindowMonthly])).
			Times(1).
			Return(nil)

		gameCache := newTestGameCache(cache)

		err := gameCache.SetLeaderboard(context.Background(), LeaderboardScopeGlobal, uuid.Nil, LeaderboardWindowMonthly, 3, "")
		require.NoError(t, err)
	})

	t.Run("server leaderboard", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		cache := mockcache.NewMockCache(ctrl)

		serverID := uuid.New()
		expectedKey := fmt.Sprintf("test-leaderboard:server:%s:weekly:3", serverID.String())

		cache.EXPECT().
			Set(gomock.Any(), gomock.Eq(expectedKey), gomock.Any(), gomock.Eq(leaderboardTTLs[LeaderboardWindowWeekly])).
			Times(1).
			Return(nil)

		gameCache := newTestGameCache(cache)

		err := gameCache.SetLeaderboard(context.Background(), LeaderboardScopeServer, serverID, LeaderboardWindowWeekly, 3, "")
		require.NoError(t, err)
	})

	t.Run("invalid ttl", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		cache := mockcache.NewMockCache(ctrl)

		serverID := uuid.New()

		gameCache := newTestGameCache(cache)

		err := gameCache.SetLeaderboard(context.Background(), LeaderboardScopeServer, serverID, "some-invalid-window", 3, "")
		require.Error(t, err)
	})
}

func TestGetLeaderboard(t *testing.T) {
	t.Run("global leaderboard", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		cache := mockcache.NewMockCache(ctrl)

		expectedKey := "test-leaderboard:global:daily:34"

		cache.EXPECT().
			Get(gomock.Any(), gomock.Eq(expectedKey), gomock.Any()).
			Times(1).
			Return(nil)

		gameCache := newTestGameCache(cache)

		err := gameCache.GetLeaderboard(context.Background(), LeaderboardScopeGlobal, uuid.Nil, LeaderboardWindowDaily, 34, "")
		require.NoError(t, err)
	})

	t.Run("server leaderboard", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		cache := mockcache.NewMockCache(ctrl)

		serverID := uuid.New()
		expectedKey := fmt.Sprintf("test-leaderboard:server:%s:daily:4", serverID.String())

		cache.EXPECT().
			Get(gomock.Any(), gomock.Eq(expectedKey), gomock.Any()).
			Times(1).
			Return(nil)

		gameCache := newTestGameCache(cache)

		err := gameCache.GetLeaderboard(context.Background(), LeaderboardScopeServer, serverID, LeaderboardWindowDaily, 4, "")
		require.NoError(t, err)
	})
}

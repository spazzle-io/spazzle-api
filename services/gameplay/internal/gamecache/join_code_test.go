package gamecache

import (
	"context"
	"testing"

	"github.com/google/uuid"
	mockcache "github.com/spazzle-io/spazzle-api/libs/common/cache/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestSetJoinCode(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cache := mockcache.NewMockCache(ctrl)

	cache.EXPECT().
		Set(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Eq(joinCodeTTL)).
		Times(1).
		Return(nil)

	gameCache := newTestGameCache(cache)

	entry := &JoinCodeEntry{
		UserID:   uuid.New(),
		ServerID: uuid.New(),
		GameID:   uuid.New(),
		Role:     "player",
	}

	joinCode, expiresAt, err := gameCache.SetJoinCode(context.Background(), entry)
	require.NoError(t, err)
	require.NotEmpty(t, joinCode)
	require.NotNil(t, expiresAt)
}

func TestGetJoinCode(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cache := mockcache.NewMockCache(ctrl)

	joinCodeEntry := JoinCodeEntry{
		UserID:   uuid.New(),
		ServerID: uuid.New(),
		GameID:   uuid.New(),
		Role:     "player",
	}

	cache.EXPECT().
		Get(gomock.Any(), gomock.Any(), gomock.Any()).
		Times(1).
		DoAndReturn(func(ctx context.Context, key string, dest *JoinCodeEntry) error {
			*dest = joinCodeEntry
			return nil
		})

	gameCache := newTestGameCache(cache)

	fetchedEntry, err := gameCache.GetJoinCodeEntry(context.Background(), "join-code")
	require.NoError(t, err)
	require.Equal(t, joinCodeEntry, *fetchedEntry)
}

func TestValidateJoinCode(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		cache := mockcache.NewMockCache(ctrl)

		serverID := uuid.New()
		gameID := uuid.New()

		cache.EXPECT().
			Get(gomock.Any(), gomock.Any(), gomock.Any()).
			Times(1).
			DoAndReturn(func(ctx context.Context, key string, dest *JoinCodeEntry) error {
				*dest = JoinCodeEntry{
					UserID:   uuid.New(),
					ServerID: serverID,
					GameID:   gameID,
					Role:     "player",
				}
				return nil
			})

		gameCache := newTestGameCache(cache)

		joinCodeEntry, err := gameCache.ValidateJoinCode(context.Background(), "join-code", serverID, gameID)
		require.NoError(t, err)
		require.NotNil(t, joinCodeEntry)
	})

	t.Run("invalid server id", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		cache := mockcache.NewMockCache(ctrl)

		serverID := uuid.New()
		gameID := uuid.New()

		cache.EXPECT().
			Get(gomock.Any(), gomock.Any(), gomock.Any()).
			Times(1).
			DoAndReturn(func(ctx context.Context, key string, dest *JoinCodeEntry) error {
				*dest = JoinCodeEntry{
					UserID:   uuid.New(),
					ServerID: uuid.New(),
					GameID:   gameID,
					Role:     "player",
				}
				return nil
			})

		gameCache := newTestGameCache(cache)

		joinCodeEntry, err := gameCache.ValidateJoinCode(context.Background(), "join-code", serverID, gameID)
		require.Error(t, err)
		require.Nil(t, joinCodeEntry)
	})

	t.Run("invalid game id", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		cache := mockcache.NewMockCache(ctrl)

		serverID := uuid.New()
		gameID := uuid.New()

		cache.EXPECT().
			Get(gomock.Any(), gomock.Any(), gomock.Any()).
			Times(1).
			DoAndReturn(func(ctx context.Context, key string, dest *JoinCodeEntry) error {
				*dest = JoinCodeEntry{
					UserID:   uuid.New(),
					ServerID: serverID,
					GameID:   uuid.New(),
					Role:     "player",
				}
				return nil
			})

		gameCache := newTestGameCache(cache)

		joinCodeEntry, err := gameCache.ValidateJoinCode(context.Background(), "join-code", serverID, gameID)
		require.Error(t, err)
		require.Nil(t, joinCodeEntry)
	})
}

func TestInvalidateJoinCode(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cache := mockcache.NewMockCache(ctrl)

	cache.EXPECT().
		Del(gomock.Any(), gomock.Any()).
		Times(1).
		Return(nil)

	gameCache := newTestGameCache(cache)

	err := gameCache.InvalidateJoinCode(context.Background(), "join-code")
	require.NoError(t, err)
}

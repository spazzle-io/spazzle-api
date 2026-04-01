package gamecache

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

var ErrCurrentGameCacheMiss = errors.New("current game cache miss")

const (
	currentGamePrefix = "current_game"
	currentGameTTL    = 5 * time.Second
)

func (gc *GameCache) GetCurrentGame(ctx context.Context, serverID uuid.UUID, dest interface{}) error {
	key := gc.currentGameKey(serverID)

	if err := gc.cache.Get(ctx, key, dest); err != nil {
		return fmt.Errorf("%w: %v", ErrCurrentGameCacheMiss, err)
	}

	return nil
}

func (gc *GameCache) SetCurrentGame(ctx context.Context, serverID uuid.UUID, data interface{}) error {
	key := gc.currentGameKey(serverID)

	if err := gc.cache.Set(ctx, key, data, currentGameTTL); err != nil {
		return fmt.Errorf("failed to cache current game: %w", err)
	}

	return nil
}

func (gc *GameCache) currentGameKey(serverID uuid.UUID) string {
	return fmt.Sprintf("%s-%s:%s", gc.config.ServiceName, currentGamePrefix, serverID.String())
}

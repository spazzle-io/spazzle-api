package gamecache

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

var ErrLeaderboardCacheMiss = errors.New("leaderboard cache miss")

const leaderboardPrefix = "leaderboard"

type LeaderboardScope string

const (
	LeaderboardScopeGlobal LeaderboardScope = "global"
	LeaderboardScopeServer LeaderboardScope = "server"
)

type LeaderboardWindow string

const (
	LeaderboardWindowDaily   LeaderboardWindow = "daily"
	LeaderboardWindowWeekly  LeaderboardWindow = "weekly"
	LeaderboardWindowMonthly LeaderboardWindow = "monthly"
)

var leaderboardTTLs = map[LeaderboardWindow]time.Duration{
	LeaderboardWindowDaily:   60 * time.Second,
	LeaderboardWindowWeekly:  5 * time.Minute,
	LeaderboardWindowMonthly: 10 * time.Minute,
}

func (gc *GameCache) GetLeaderboard(ctx context.Context, scope LeaderboardScope, scopeID uuid.UUID, window LeaderboardWindow, page int32, dest interface{}) error {
	key := gc.leaderboardKey(scope, scopeID, window, page)

	if err := gc.cache.Get(ctx, key, dest); err != nil {
		return fmt.Errorf("%w: %v", ErrLeaderboardCacheMiss, err)
	}

	return nil
}

func (gc *GameCache) SetLeaderboard(ctx context.Context, scope LeaderboardScope, scopeID uuid.UUID, window LeaderboardWindow, page int32, data interface{}) error {
	key := gc.leaderboardKey(scope, scopeID, window, page)

	ttl, ok := leaderboardTTLs[window]
	if !ok {
		return fmt.Errorf("unknown leaderboard window: %s", window)
	}

	if err := gc.cache.Set(ctx, key, data, ttl); err != nil {
		return fmt.Errorf("failed to cache leaderboard: %w", err)
	}

	return nil
}

func (gc *GameCache) leaderboardKey(scope LeaderboardScope, scopeID uuid.UUID, window LeaderboardWindow, page int32) string {
	if scope == LeaderboardScopeGlobal {
		return fmt.Sprintf("%s-%s:%s:%s:%d", gc.config.ServiceName, leaderboardPrefix, scope, window, page)
	}
	return fmt.Sprintf("%s-%s:%s:%s:%s:%d", gc.config.ServiceName, leaderboardPrefix, scope, scopeID.String(), window, page)
}

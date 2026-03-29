package gamecache

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	commonUtil "github.com/spazzle-io/spazzle-api/libs/common/util"
)

var (
	ErrInvalidJoinCode        = errors.New("invalid join code")
	ErrJoinCodeServerMismatch = errors.New("join code server mismatch")
	ErrJoinCodeGameMismatch   = errors.New("join code game mismatch")
)

const (
	joinCodeLength = 8
	joinCodePrefix = "join_code"
	joinCodeTTL    = 5 * time.Minute
)

type JoinCodeEntry struct {
	UserID   uuid.UUID
	ServerID uuid.UUID
	GameID   uuid.UUID
	Role     string
}

func (gc *GameCache) SetJoinCode(ctx context.Context, entry *JoinCodeEntry) (string, time.Time, error) {
	joinCode, err := commonUtil.GenerateRandomAlphanumericString(joinCodeLength)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("could not generate join code: %w", err)
	}

	expiresAt := time.Now().UTC().Add(joinCodeTTL)

	key := gc.joinCodeKey(joinCode)
	if err := gc.cache.Set(ctx, key, entry, joinCodeTTL); err != nil {
		return "", time.Time{}, fmt.Errorf("could not cache join code: %w", err)
	}

	return joinCode, expiresAt, nil
}

func (gc *GameCache) GetJoinCodeEntry(ctx context.Context, joinCode string) (*JoinCodeEntry, error) {
	key := gc.joinCodeKey(joinCode)

	var entry JoinCodeEntry
	if err := gc.cache.Get(ctx, key, &entry); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidJoinCode, err)
	}

	return &entry, nil
}

func (gc *GameCache) ValidateJoinCode(ctx context.Context, joinCode string, serverID uuid.UUID, gameID uuid.UUID) (*JoinCodeEntry, error) {
	entry, err := gc.GetJoinCodeEntry(ctx, joinCode)
	if err != nil {
		return nil, err
	}

	if entry.ServerID != serverID {
		return nil, ErrJoinCodeServerMismatch
	}

	if entry.GameID != gameID {
		return nil, ErrJoinCodeGameMismatch
	}

	return entry, nil
}

func (gc *GameCache) InvalidateJoinCode(ctx context.Context, joinCode string) error {
	key := gc.joinCodeKey(joinCode)

	if err := gc.cache.Del(ctx, key); err != nil {
		return fmt.Errorf("could not invalidate join code: %w", err)
	}

	return nil
}

func (gc *GameCache) joinCodeKey(joinCode string) string {
	return fmt.Sprintf("%s-%s:%s", gc.config.ServiceName, joinCodePrefix, joinCode)
}

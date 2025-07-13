package handler

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/spazzle-io/spazzle-api/libs/common/middleware"
	db "github.com/spazzle-io/spazzle-api/services/auth/internal/db/sqlc"
	"github.com/spazzle-io/spazzle-api/services/auth/internal/token"
	"github.com/spazzle-io/spazzle-api/services/auth/internal/util"
)

type Session struct {
	ID                  string
	AccessToken         string
	AccessTokenPayload  *token.Payload
	RefreshToken        string
	RefreshTokenPayload *token.Payload
	ExpiresAt           time.Time
}

func NewSession(
	ctx context.Context,
	userId uuid.UUID,
	walletAddress string,
	role token.Role,
	config util.Config,
	tokenMaker token.Maker,
	store db.Store,
) (*Session, error) {
	accessToken, accessTokenPayload, err := tokenMaker.CreateToken(
		userId, walletAddress, role, token.AccessToken, config.AccessTokenDuration,
	)
	if err != nil {
		return nil, fmt.Errorf("could not create access token: %w", err)
	}

	refreshToken, refreshTokenPayload, err := tokenMaker.CreateToken(
		userId, walletAddress, role, token.RefreshToken, config.RefreshTokenDuration,
	)
	if err != nil {
		return nil, fmt.Errorf("could not create refresh token: %w", err)
	}

	clientIP, ok := ctx.Value(middleware.ClientIP).(string)
	if !ok {
		clientIP = "unknown"
	}

	userAgent, ok := ctx.Value(middleware.UserAgent).(string)
	if !ok {
		userAgent = "unknown"
	}

	session, err := store.CreateSession(ctx, db.CreateSessionParams{
		ID:            refreshTokenPayload.ID,
		UserID:        userId,
		WalletAddress: walletAddress,
		RefreshToken:  refreshToken,
		UserAgent:     userAgent,
		ClientIp:      clientIP,
		ExpiresAt:     refreshTokenPayload.ExpiresAt,
	})
	if err != nil {
		return nil, fmt.Errorf("could not create db session: %w", err)
	}

	return &Session{
		ID:                  session.ID.String(),
		AccessToken:         accessToken,
		AccessTokenPayload:  accessTokenPayload,
		RefreshToken:        refreshToken,
		RefreshTokenPayload: refreshTokenPayload,
		ExpiresAt:           session.ExpiresAt,
	}, nil
}

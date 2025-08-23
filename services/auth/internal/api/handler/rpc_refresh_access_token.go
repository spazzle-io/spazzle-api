package handler

import (
	"context"

	"github.com/rs/zerolog/log"
	"github.com/spazzle-io/spazzle-api/services/auth/internal/api/middleware"
	"github.com/spazzle-io/spazzle-api/services/auth/internal/token"
	pb "github.com/spazzle-io/spazzle-api/services/proto/auth/auth/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (h *Handler) RefreshAccessToken(
	ctx context.Context,
	_ *pb.RefreshAccessTokenRequest,
) (*pb.RefreshAccessTokenResponse, error) {
	tkPayload, err := middleware.AuthorizeToken(ctx, h.tokenMaker, token.RefreshToken, nil)
	if err != nil {
		log.Error().Err(err).Msg("could not authorize refresh token")
		return nil, status.Error(codes.Unauthenticated, UnauthorizedAccessError)
	}

	logger := log.With().Str("user_id", tkPayload.UserId.String()).Logger()

	session, err := h.store.GetSessionById(ctx, tkPayload.ID)
	if err != nil {
		logger.Error().Err(err).Msg("could not get session")
		return nil, status.Error(codes.Internal, InternalServerError)
	}

	if session.IsRevoked {
		logger.Error().Str("session_id", session.ID.String()).Msg("session is revoked")
		return nil, status.Error(codes.PermissionDenied, UnauthorizedAccessError)
	}

	accessToken, accessTokenPayload, err := h.tokenMaker.CreateToken(
		tkPayload.UserId, tkPayload.WalletAddress, tkPayload.Role, token.AccessToken, h.config.AccessTokenDuration,
	)
	if err != nil {
		logger.Error().Err(err).Msg("could not create access token")
		return nil, status.Error(codes.Internal, InternalServerError)
	}

	res := &pb.RefreshAccessTokenResponse{
		Session: &pb.Session{
			SessionId:             session.ID.String(),
			AccessToken:           accessToken,
			RefreshToken:          session.RefreshToken,
			AccessTokenExpiresAt:  timestamppb.New(accessTokenPayload.ExpiresAt),
			RefreshTokenExpiresAt: timestamppb.New(session.ExpiresAt),
			TokenType:             authorizationBearer,
		},
	}

	logger.Info().Msg("refreshed access token successfully")

	return res, nil
}

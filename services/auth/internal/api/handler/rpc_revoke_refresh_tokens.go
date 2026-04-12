package handler

import (
	"context"

	"github.com/rs/zerolog/log"
	"github.com/spazzle-io/spazzle-api/services/auth/internal/api/middleware"
	"github.com/spazzle-io/spazzle-api/services/auth/internal/token"
	pb "github.com/spazzle-io/spazzle-api/services/proto/auth/auth/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (h *Handler) RevokeRefreshTokens(
	ctx context.Context,
	_ *pb.RevokeRefreshTokensRequest,
) (*pb.RevokeRefreshTokensResponse, error) {
	tkPayload, err := middleware.AuthorizeToken(ctx, h.tokenMaker, token.AccessToken, nil)
	if err != nil {
		log.Error().Err(err).Msg("could not authorize token")
		return nil, status.Error(codes.Unauthenticated, UnauthorizedAccessError)
	}

	logger := log.With().Str("user_id", tkPayload.UserID.String()).Logger()

	ct, err := h.store.RevokeSessions(ctx, tkPayload.UserID)
	if err != nil {
		logger.Error().Err(err).Msg("could not revoke sessions")
		return nil, status.Error(codes.Internal, InternalServerError)
	}

	res := &pb.RevokeRefreshTokensResponse{
		NumSessionsRevoked: ct.RowsAffected(),
	}

	logger.Info().Int64("num_sessions_revoked", ct.RowsAffected()).Msg("successfully revoked refresh tokens")

	return res, nil
}

package handler

import (
	"context"

	"buf.build/go/protovalidate"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/spazzle-io/spazzle-api/services/auth/internal/api/middleware"
	"github.com/spazzle-io/spazzle-api/services/auth/internal/token"
	pb "github.com/spazzle-io/spazzle-api/services/proto/auth/auth/v1"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (h *Handler) RevokeRefreshTokens(
	ctx context.Context,
	req *pb.RevokeRefreshTokensRequest,
) (*pb.RevokeRefreshTokensResponse, error) {
	logger := log.With().Str("user_id", req.GetUserId()).Logger()

	violations := validateRevokeRefreshTokensRequest(req)
	if violations != nil {
		return nil, invalidArgumentError(violations)
	}

	userId, err := uuid.Parse(req.GetUserId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, InvalidUserIdError)
	}

	_, err = middleware.AuthorizeToken(ctx, userId, h.tokenMaker, token.AccessToken, nil)
	if err != nil {
		logger.Error().Err(err).Msg("could not authorize token")
		return nil, status.Error(codes.Unauthenticated, UnauthorizedAccessError)
	}

	ct, err := h.store.RevokeSessions(ctx, userId)
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

func validateRevokeRefreshTokensRequest(
	req *pb.RevokeRefreshTokensRequest,
) (violations []*errdetails.BadRequest_FieldViolation) {
	if err := protovalidate.Validate(req); err != nil {
		violations = append(violations, protovalidateViolation(err)...)
	}

	return violations
}

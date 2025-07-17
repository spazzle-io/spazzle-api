package handler

import (
	"context"
	"errors"

	"buf.build/go/protovalidate"

	"github.com/ethereum/go-ethereum/common"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/spazzle-io/spazzle-api/services/auth/internal/api/middleware"
	"github.com/spazzle-io/spazzle-api/services/auth/internal/token"
	pb "github.com/spazzle-io/spazzle-api/services/proto/auth/auth/v1"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (h *Handler) RefreshAccessToken(
	ctx context.Context,
	req *pb.RefreshAccessTokenRequest,
) (*pb.RefreshAccessTokenResponse, error) {
	logger := log.With().Str("user_id", req.GetUserId()).Logger()

	violations := validateRefreshAccessTokenRequest(req)
	if violations != nil {
		return nil, invalidArgumentError(violations)
	}

	userId, err := uuid.Parse(req.GetUserId())
	if err != nil {
		logger.Error().Err(err).Msg("could not parse user id")
		return nil, status.Error(codes.InvalidArgument, InvalidUserIdError)
	}

	tkPayload, err := middleware.AuthorizeToken(ctx, userId, h.tokenMaker, token.RefreshToken, nil)
	if err != nil {
		logger.Error().Err(err).Msg("could not authorize token")
		return nil, status.Error(codes.Unauthenticated, UnauthorizedAccessError)
	}

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
		userId, req.GetWalletAddress(), tkPayload.Role, token.AccessToken, h.config.AccessTokenDuration,
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

func validateRefreshAccessTokenRequest(
	req *pb.RefreshAccessTokenRequest,
) (violations []*errdetails.BadRequest_FieldViolation) {
	if err := protovalidate.Validate(req); err != nil {
		violations = append(violations, protovalidateViolation(err)...)
	}

	if isHexAddress := common.IsHexAddress(req.GetWalletAddress()); !isHexAddress {
		violations = append(violations, fieldViolation("walletAddress", errors.New("must be a hex address")))
	}

	return violations
}

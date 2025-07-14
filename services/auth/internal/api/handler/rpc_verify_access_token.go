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
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (h *Handler) VerifyAccessToken(
	ctx context.Context,
	req *pb.VerifyAccessTokenRequest,
) (*pb.VerifyAccessTokenResponse, error) {
	logger := log.With().Str("user_id", req.GetUserId()).Logger()

	violations := validateVerifyAccessTokenRequest(req)
	if violations != nil {
		return nil, invalidArgumentError(violations)
	}

	userId, err := uuid.Parse(req.GetUserId())
	if err != nil {
		logger.Error().Err(err).Msg("could not parse user id")
		return nil, status.Error(codes.InvalidArgument, InvalidUserIdError)
	}

	tkPayload, err := middleware.AuthorizeToken(ctx, userId, h.tokenMaker, token.AccessToken, []token.Role{token.User})
	if err != nil {
		logger.Error().Err(err).Msg("could not authorize token")
		return nil, status.Error(codes.Unauthenticated, UnauthorizedAccessError)
	}

	resPayloadRole := pb.AccessTokenPayload_ROLE_UNSPECIFIED
	switch tkPayload.Role {
	case token.User:
		resPayloadRole = pb.AccessTokenPayload_ROLE_USER
	case token.Admin:
		resPayloadRole = pb.AccessTokenPayload_ROLE_ADMIN
	}

	res := &pb.VerifyAccessTokenResponse{
		AccessToken: &pb.AccessTokenPayload{
			Id:            tkPayload.ID.String(),
			UserId:        tkPayload.UserId.String(),
			WalletAddress: tkPayload.WalletAddress,
			Role:          resPayloadRole,
			IssuedAt:      timestamppb.New(tkPayload.IssuedAt),
			ExpiresAt:     timestamppb.New(tkPayload.ExpiresAt),
		},
	}

	return res, nil
}

func validateVerifyAccessTokenRequest(
	req *pb.VerifyAccessTokenRequest,
) (violations []*errdetails.BadRequest_FieldViolation) {
	if err := protovalidate.Validate(req); err != nil {
		violations = append(violations, protovalidateViolation(err)...)
	}

	return violations
}

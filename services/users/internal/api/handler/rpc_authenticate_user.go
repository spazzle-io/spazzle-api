package handler

import (
	"context"

	"buf.build/go/protovalidate"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	authPb "github.com/spazzle-io/spazzle-api/services/proto/auth/auth/v1"
	pb "github.com/spazzle-io/spazzle-api/services/proto/users/users/v1"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (h *Handler) AuthenticateUser(ctx context.Context, req *pb.AuthenticateUserRequest) (*pb.AuthenticateUserResponse, error) {
	logger := log.With().Str("user_id", req.GetUserId()).Logger()

	violations := validateAuthenticateUserRequest(req)
	if violations != nil {
		return nil, invalidArgumentError(violations)
	}

	userId, err := uuid.Parse(req.GetUserId())
	if err != nil {
		logger.Error().Err(err).Str("user_id", req.GetUserId()).Msg("invalid user id")
		return nil, status.Error(codes.InvalidArgument, InvalidUserIdError)
	}

	user, err := h.store.GetUserById(ctx, userId)
	if err != nil {
		logger.Error().Err(err).Str("user_id", req.GetUserId()).Msg("user does not exist")
		return nil, status.Error(codes.NotFound, UserNotFoundError)
	}

	authenticateRequest := &authPb.AuthenticateRequest{
		WalletAddress: user.WalletAddress,
		UserId:        user.ID.String(),
		Signature:     req.GetSignature(),
	}

	authenticateRes, err := h.authService.Authenticate(ctx, h.config.ServiceName, authenticateRequest)
	if err != nil {
		logger.Error().Err(err).Msg("could not authenticate user")
		return nil, status.Error(codes.Internal, InternalServerError)
	}

	response := &pb.AuthenticateUserResponse{
		User: &pb.User{
			UserId:        user.ID.String(),
			WalletAddress: user.WalletAddress,
			GamerTag:      user.GamerTag.String,
			CreatedAt:     timestamppb.New(user.CreatedAt),
		},
		Session: &pb.Session{
			SessionId:             authenticateRes.Session.SessionId,
			AccessToken:           authenticateRes.Session.AccessToken,
			RefreshToken:          authenticateRes.Session.RefreshToken,
			AccessTokenExpiresAt:  authenticateRes.Session.AccessTokenExpiresAt,
			RefreshTokenExpiresAt: authenticateRes.Session.RefreshTokenExpiresAt,
			TokenType:             authenticateRes.Session.TokenType,
		},
	}

	logger.Info().Msg("user authenticated successfully")

	return response, nil
}

func validateAuthenticateUserRequest(req *pb.AuthenticateUserRequest) (violations []*errdetails.BadRequest_FieldViolation) {
	if err := protovalidate.Validate(req); err != nil {
		violations = append(violations, protovalidateViolation(err)...)
	}

	return violations
}

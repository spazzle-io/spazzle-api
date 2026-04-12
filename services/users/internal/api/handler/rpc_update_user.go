package handler

import (
	"context"

	"buf.build/go/protovalidate"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog/log"
	authPb "github.com/spazzle-io/spazzle-api/services/proto/auth/auth/v1"
	pb "github.com/spazzle-io/spazzle-api/services/proto/users/users/v1"
	db "github.com/spazzle-io/spazzle-api/services/users/internal/db/sqlc"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (h *Handler) UpdateUser(ctx context.Context, req *pb.UpdateUserRequest) (*pb.UpdateUserResponse, error) {
	logger := log.With().Str("user_id", req.GetId()).Logger()

	violations := validateUpdateUserRequest(req)
	if violations != nil {
		return nil, invalidArgumentError(violations)
	}

	userId, err := uuid.Parse(req.GetId())
	if err != nil {
		logger.Info().Err(err).Msg("invalid user id")
		return nil, status.Error(codes.InvalidArgument, InvalidUserIdError)
	}

	_, err = h.authService.VerifyAccessToken(ctx, h.config, &authPb.VerifyAccessTokenRequest{})
	if err != nil {
		logger.Error().Err(err).Msg("access token verification failed")
		return nil, status.Error(codes.Unauthenticated, UnauthorizedAccessError)
	}

	params := db.UpdateUserParams{
		UserID: userId,
		GamerTag: pgtype.Text{
			String: req.GetGamerTag().GetValue(),
			Valid:  req.GetGamerTag() != nil,
		},
	}
	user, err := h.store.UpdateUser(ctx, params)
	if err != nil {
		logger.Info().Err(err).Msg("failed to update user")
		return nil, status.Error(codes.Internal, InternalServerError)
	}

	response := &pb.UpdateUserResponse{
		User: &pb.User{
			Id:            user.ID.String(),
			WalletAddress: user.WalletAddress,
			GamerTag:      user.GamerTag.String,
			CreatedAt:     timestamppb.New(user.CreatedAt),
		},
	}

	logger.Info().Msg("successfully updated user")

	return response, nil
}

func validateUpdateUserRequest(req *pb.UpdateUserRequest) (violations []*errdetails.BadRequest_FieldViolation) {
	if err := protovalidate.Validate(req); err != nil {
		violations = append(violations, protovalidateViolation(err)...)
	}

	return violations
}

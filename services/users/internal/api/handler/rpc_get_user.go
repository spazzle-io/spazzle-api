package handler

import (
	"context"

	"buf.build/go/protovalidate"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	pb "github.com/spazzle-io/spazzle-api/services/proto/users/users/v1"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (h *Handler) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.GetUserResponse, error) {
	logger := log.With().Str("user_id", req.GetId()).Logger()

	violations := validateGetUserRequest(req)
	if violations != nil {
		return nil, invalidArgumentError(violations)
	}

	userId, err := uuid.Parse(req.GetId())
	if err != nil {
		logger.Error().Err(err).Msg("invalid user id")
		return nil, status.Error(codes.InvalidArgument, InvalidUserIdError)
	}

	user, err := h.store.GetUserById(ctx, userId)
	if err != nil {
		logger.Error().Err(err).Msg("could not get user")
		return nil, status.Error(codes.NotFound, UserNotFoundError)
	}

	response := &pb.GetUserResponse{
		User: &pb.User{
			Id:            user.ID.String(),
			WalletAddress: user.WalletAddress,
			GamerTag:      user.GamerTag.String,
			CreatedAt:     timestamppb.New(user.CreatedAt),
		},
	}

	logger.Info().Msg("successfully retrieved user")

	return response, nil
}

func validateGetUserRequest(req *pb.GetUserRequest) (violations []*errdetails.BadRequest_FieldViolation) {
	if err := protovalidate.Validate(req); err != nil {
		violations = append(violations, protovalidateViolation(err)...)
	}

	return violations
}

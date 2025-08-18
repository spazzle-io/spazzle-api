package server

import (
	"context"
	"errors"

	"buf.build/go/protovalidate"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/api/handler"
	db "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/sqlc"
	pb "github.com/spazzle-io/spazzle-api/services/proto/gameplay/gameplay/v1"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (h *Handler) GetUserServerPermissions(ctx context.Context, req *pb.GetUserServerPermissionsRequest) (*pb.GetUserServerPermissionsResponse, error) {
	violations := validateGetUserServerPermissionsRequest(req)
	if violations != nil {
		return nil, handler.InvalidArgumentError(violations)
	}

	userId, err := uuid.Parse(req.GetUserId())
	if err != nil {
		log.Error().Err(err).Msg("invalid user id")
		return nil, status.Error(codes.InvalidArgument, handler.InvalidUserIdError)
	}

	serverId, err := uuid.Parse(req.GetServerId())
	if err != nil {
		log.Error().Err(err).Msg("invalid server id")
		return nil, status.Error(codes.InvalidArgument, handler.InvalidServerIdError)
	}

	params := db.GetServerUserPermissionsParams{
		UserID:   userId,
		ServerID: serverId,
	}

	permissions, err := h.store.GetServerUserPermissions(ctx, params)
	if err != nil && !errors.Is(err, db.RecordNotFoundError) {
		log.Error().Err(err).Msg("failed to get server user permissions")
		return nil, status.Error(codes.Internal, handler.InternalServerError)
	}

	response := &pb.GetUserServerPermissionsResponse{
		IsOwner:                permissions.IsOwner,
		IsAdmin:                permissions.IsAdmin,
		HasElevatedPermissions: permissions.HasElevatedPermissions,
	}

	log.Info().Msg("got server user permissions successfully")

	return response, nil
}

func validateGetUserServerPermissionsRequest(req *pb.GetUserServerPermissionsRequest) (violations []*errdetails.BadRequest_FieldViolation) {
	if err := protovalidate.Validate(req); err != nil {
		violations = append(violations, handler.ProtovalidateViolation(err)...)
	}

	return violations
}

package server_admin

import (
	"context"
	"errors"

	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/api/middleware"

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

func (h *Handler) RemoveServerAdmin(ctx context.Context, req *pb.RemoveServerAdminRequest) (*pb.RemoveServerAdminResponse, error) {
	logger := log.With().Str("user_to_remove", req.GetUserId()).Str("server_id", req.GetServerId()).Logger()

	violations := validateRemoveServerAdminRequest(req)
	if violations != nil {
		return nil, handler.InvalidArgumentError(violations)
	}

	serverUserCtx, err := middleware.ResolveServerUserContext(
		ctx, h.Config, req.GetServerId(), h.Store, h.AuthService,
	)
	if err != nil {
		return nil, err
	}

	logger = logger.With().Str("user_id", serverUserCtx.UserId.String()).Logger()

	userToRemove, err := uuid.Parse(req.GetUserId())
	if err != nil {
		logger.Error().Err(err).Msg("invalid user id")
		return nil, status.Error(codes.InvalidArgument, handler.InvalidUserIdError)
	}

	if !serverUserCtx.UserServerPermissions.IsOwner {
		logger.Error().Err(err).Msg("user does not have permission to remove a server admin")
		return nil, status.Error(codes.Unauthenticated, handler.UnauthorizedAccessError)
	}

	params := db.RemoveServerAdminTxParams{
		UserId:   userToRemove,
		ServerId: serverUserCtx.ServerId,
	}
	err = h.Store.RemoveServerAdminTx(ctx, params)
	if err != nil {
		logger.Error().Err(err).Msg("failed to remove server admin")
		return nil, handleRemoveServerAdminTxError(err)
	}

	response := &pb.RemoveServerAdminResponse{
		UserId: userToRemove.String(),
	}

	logger.Info().Msg("successfully removed server admin")

	return response, nil
}

func handleRemoveServerAdminTxError(err error) error {
	if errors.Is(err, db.ErrServerNotfound) {
		return status.Error(codes.NotFound, handler.ServerNotFoundError)
	}

	return status.Error(codes.Internal, handler.InternalServerError)
}

func validateRemoveServerAdminRequest(req *pb.RemoveServerAdminRequest) (violations []*errdetails.BadRequest_FieldViolation) {
	if err := protovalidate.Validate(req); err != nil {
		violations = append(violations, handler.ProtovalidateViolation(err)...)
	}

	return violations
}

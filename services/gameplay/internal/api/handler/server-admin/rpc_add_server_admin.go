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
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (h *Handler) AddServerAdmin(ctx context.Context, req *pb.AddServerAdminRequest) (*pb.AddServerAdminResponse, error) {
	logger := log.With().Str("user_to_add", req.GetUserId()).Str("server_id", req.GetServerId()).Logger()

	violations := validateAddServerAdminRequest(req)
	if violations != nil {
		return nil, handler.InvalidArgumentError(violations)
	}

	serverUserCtx, err := middleware.ResolveServerUserContext(
		ctx, req.GetServerId(), h.Config.ServiceName, h.Store, h.AuthService,
	)
	if err != nil {
		return nil, err
	}

	logger = logger.With().Str("user_id", serverUserCtx.UserId.String()).Logger()

	userToAdd, err := uuid.Parse(req.GetUserId())
	if err != nil {
		logger.Error().Err(err).Msg("invalid user id")
		return nil, status.Error(codes.InvalidArgument, handler.InvalidUserIdError)
	}

	if !serverUserCtx.UserServerPermissions.IsOwner {
		logger.Error().Err(err).Msg("user does not have permission to add a server admin")
		return nil, status.Error(codes.Unauthenticated, handler.UnauthorizedAccessError)
	}

	params := db.AddServerAdminTxParams{
		UserId:   userToAdd,
		ServerId: serverUserCtx.ServerId,
	}
	txResult, err := h.Store.AddServerAdminTx(ctx, params)
	if err != nil {
		logger.Error().Err(err).Msg("failed to add server admin")
		return nil, handleAddServerAdminTxError(err)
	}

	response := &pb.AddServerAdminResponse{
		Admin: &pb.ServerAdmin{
			ServerId: txResult.ServerAdmin.ServerID.String(),
			UserId:   txResult.ServerAdmin.UserID.String(),
			AddedAt:  timestamppb.New(txResult.ServerAdmin.AddedAt),
		},
	}

	logger.Info().Msg("successfully added server admin")

	return response, nil
}

func handleAddServerAdminTxError(err error) error {
	switch {
	case errors.Is(err, db.ErrServerNotfound):
		return status.Error(codes.NotFound, handler.ServerNotFoundError)
	case errors.Is(err, db.ErrUserAlreadyAdmin):
		return status.Error(codes.AlreadyExists, err.Error())
	default:
		return status.Error(codes.Internal, handler.InternalServerError)
	}
}

func validateAddServerAdminRequest(req *pb.AddServerAdminRequest) (violations []*errdetails.BadRequest_FieldViolation) {
	if err := protovalidate.Validate(req); err != nil {
		violations = append(violations, handler.ProtovalidateViolation(err)...)
	}

	return violations
}

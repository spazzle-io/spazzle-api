package word

import (
	"context"

	"buf.build/go/protovalidate"

	"github.com/rs/zerolog/log"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/api/handler"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/api/middleware"
	db "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/sqlc"
	pb "github.com/spazzle-io/spazzle-api/services/proto/gameplay/gameplay/v1"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (h *Handler) RemoveWords(ctx context.Context, req *pb.RemoveWordsRequest) (*pb.RemoveWordsResponse, error) {
	logger := log.With().Str("server_id", req.GetServerId()).Logger()

	violations := validateRemoveServerWordsRequest(req)
	if violations != nil {
		return nil, handler.InvalidArgumentError(violations)
	}

	serverUserCtx, err := middleware.ResolveServerUserContext(
		ctx, req.GetServerId(), h.config.ServiceName, h.store, h.authService,
	)
	if err != nil {
		return nil, err
	}

	logger = logger.With().Str("user_id", serverUserCtx.UserId.String()).Logger()

	if !serverUserCtx.UserServerPermissions.HasElevatedPermissions {
		logger.Error().Err(err).Msg("user does not have permission to remove server words")
		return nil, status.Error(codes.Unauthenticated, handler.UnauthorizedAccessError)
	}

	params := db.RemoveServerWordsTxParams{
		ServerId: serverUserCtx.ServerId,
		Words:    req.GetWords(),
	}
	txResult, err := h.store.RemoveServerWordsTx(ctx, params)
	if err != nil {
		logger.Error().Err(err).Msg("failed to remove server words")
		return nil, status.Error(codes.Internal, handler.InternalServerError)
	}

	response := &pb.RemoveWordsResponse{
		NumWordsRemoved: txResult.NumWordsRemoved,
	}

	logger.Info().Msg("successfully removed server words")

	return response, nil
}

func validateRemoveServerWordsRequest(req *pb.RemoveWordsRequest) (violations []*errdetails.BadRequest_FieldViolation) {
	if err := protovalidate.Validate(req); err != nil {
		violations = append(violations, handler.ProtovalidateViolation(err)...)
	}

	return violations
}

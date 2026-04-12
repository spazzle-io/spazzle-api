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

func (h *Handler) AddWords(ctx context.Context, req *pb.AddWordsRequest) (*pb.AddWordsResponse, error) {
	logger := log.With().Str("server_id", req.GetServerId()).Logger()

	violations := validateAddServerWordsRequest(req)
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

	if !serverUserCtx.UserServerPermissions.HasElevatedPermissions {
		logger.Error().Err(err).Msg("user does not have permission to add server words")
		return nil, status.Error(codes.Unauthenticated, handler.UnauthorizedAccessError)
	}

	params := db.AddServerWordsTxParams{
		ServerId: serverUserCtx.ServerId,
		Words:    req.GetWords(),
	}
	txResult, err := h.Store.AddServerWordsTx(ctx, params)
	if err != nil {
		logger.Error().Err(err).Msg("failed to add server words")
		return nil, status.Error(codes.Internal, handler.InternalServerError)
	}

	response := &pb.AddWordsResponse{
		NumWordsAdded: txResult.NumWordsAdded,
	}

	logger.Info().Msg("successfully added server words")

	return response, nil
}

func validateAddServerWordsRequest(req *pb.AddWordsRequest) (violations []*errdetails.BadRequest_FieldViolation) {
	if err := protovalidate.Validate(req); err != nil {
		violations = append(violations, handler.ProtovalidateViolation(err)...)
	}

	return violations
}

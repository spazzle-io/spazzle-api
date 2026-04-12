package word

import (
	"context"

	"buf.build/go/protovalidate"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog/log"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/api/handler"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/api/middleware"
	db "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/sqlc"
	pb "github.com/spazzle-io/spazzle-api/services/proto/gameplay/gameplay/v1"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (h *Handler) ListWords(ctx context.Context, req *pb.ListWordsRequest) (*pb.ListWordsResponse, error) {
	logger := log.With().Str("server_id", req.GetServerId()).Logger()

	violations := validateListServerWordsRequest(req)
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
		logger.Error().Err(err).Msg("user does not have permission to list server words")
		return nil, status.Error(codes.Unauthenticated, handler.UnauthorizedAccessError)
	}

	afterId, err := uuid.Parse(req.GetAfterId().GetValue())
	if err != nil && req.GetAfterId() != nil {
		log.Error().Err(err).Msg("invalid after id")
		return nil, status.Error(codes.InvalidArgument, handler.InvalidAfterIdError)
	}

	pageSize := req.GetPageSize().GetValue()
	if pageSize <= 0 || pageSize > handler.MaxPageSize {
		pageSize = handler.DefaultPageSize
	}

	params := db.ListWordsParams{
		ServerID: serverUserCtx.ServerId,
		PageSize: pageSize,
		AfterID: pgtype.UUID{
			Bytes: afterId,
			Valid: req.GetAfterId() != nil,
		},
		AfterAddedAt: pgtype.Timestamptz{
			Time:  req.GetAfterAddedAt().AsTime(),
			Valid: req.GetAfterAddedAt().IsValid(),
		},
	}

	server, err := h.Store.GetServerById(ctx, serverUserCtx.ServerId)
	if err != nil {
		log.Error().Err(err).Msg("failed to get server")
		return nil, handler.HandleServerDBError(err)
	}

	words, err := h.Store.ListWords(ctx, params)
	if err != nil {
		logger.Error().Err(err).Msg("failed to list words")
		return nil, status.Error(codes.Internal, handler.InternalServerError)
	}

	pbWords := mapDBWordsToPb(words)

	cursor := &pb.ListWordsCursor{
		PageSize: pageSize,
	}
	if n := len(words); n > 0 {
		last := words[n-1]
		cursor.AfterId = last.ID.String()
		cursor.AfterAddedAt = timestamppb.New(last.AddedAt)
	}

	response := &pb.ListWordsResponse{
		Words:      pbWords,
		TotalCount: int64(server.NumCustomWords),
		Cursor:     cursor,
	}

	log.Info().Msg("fetched server words successfully")

	return response, nil
}

func validateListServerWordsRequest(req *pb.ListWordsRequest) (violations []*errdetails.BadRequest_FieldViolation) {
	if err := protovalidate.Validate(req); err != nil {
		violations = append(violations, handler.ProtovalidateViolation(err)...)
	}

	return violations
}

package game

import (
	"context"

	"buf.build/go/protovalidate"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog/log"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/api/handler"
	db "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/sqlc"
	pb "github.com/spazzle-io/spazzle-api/services/proto/gameplay/gameplay/v1"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (h *Handler) ListServerGames(ctx context.Context, req *pb.ListServerGamesRequest) (*pb.ListServerGamesResponse, error) {
	violations := validateListServerGamesRequest(req)
	if violations != nil {
		return nil, handler.InvalidArgumentError(violations)
	}

	logger := log.With().Str("server_id", req.GetServerId()).Logger()

	afterId, err := uuid.Parse(req.GetAfterId().GetValue())
	if err != nil && req.GetAfterId() != nil {
		log.Error().Err(err).Msg("invalid after id")
		return nil, status.Error(codes.InvalidArgument, handler.InvalidAfterIdError)
	}

	serverID, err := uuid.Parse(req.GetServerId())
	if err != nil {
		logger.Error().Err(err).Msg("invalid server id")
		return nil, status.Error(codes.InvalidArgument, handler.InvalidServerIdError)
	}

	pageSize := req.GetPageSize().GetValue()
	if pageSize <= 0 || pageSize > handler.MaxPageSize {
		pageSize = handler.DefaultPageSize
	}

	params := db.ListServerGamesParams{
		ServerID: serverID,
		PageSize: pageSize,
		AfterID: pgtype.UUID{
			Bytes: afterId,
			Valid: req.GetAfterId() != nil,
		},
		AfterEndedAt: pgtype.Timestamptz{
			Time:  req.GetAfterEndedAt().AsTime(),
			Valid: req.GetAfterEndedAt().IsValid(),
		},
	}

	serverGames, err := h.store.ListServerGames(ctx, params)
	if err != nil {
		logger.Error().Err(err).Msg("failed to fetch server games")
		return nil, status.Error(codes.Internal, handler.InternalServerError)
	}

	totalCount, err := h.store.GetTotalServerGamesCount(ctx, serverID)
	if err != nil {
		logger.Error().Err(err).Msg("failed to fetch server games count")
		return nil, status.Error(codes.Internal, handler.InternalServerError)
	}

	pbServerGames, err := mapGamesToPb(serverGames)
	if err != nil {
		logger.Error().Err(err).Msg("failed to map server games to pb")
		return nil, status.Error(codes.Internal, handler.InternalServerError)
	}

	cursor := &pb.GamesCursor{
		PageSize: pageSize,
	}
	if n := len(serverGames); n > 0 {
		last := serverGames[n-1]
		cursor.AfterId = last.ID.String()
		cursor.AfterEndedAt = timestamppb.New(last.EndedAt)
	}

	return &pb.ListServerGamesResponse{
		Games:      pbServerGames,
		TotalCount: totalCount,
		Cursor:     cursor,
	}, nil
}

func validateListServerGamesRequest(req *pb.ListServerGamesRequest) (violations []*errdetails.BadRequest_FieldViolation) {
	if err := protovalidate.Validate(req); err != nil {
		violations = append(violations, handler.ProtovalidateViolation(err)...)
	}

	return violations
}

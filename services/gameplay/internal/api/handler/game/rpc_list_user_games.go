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

func (h *Handler) ListUserGames(ctx context.Context, req *pb.ListUserGamesRequest) (*pb.ListUserGamesResponse, error) {
	violations := validateListUserGamesRequest(req)
	if violations != nil {
		return nil, handler.InvalidArgumentError(violations)
	}

	logger := log.With().Str("user_id", req.GetUserId()).Logger()

	afterId, err := uuid.Parse(req.GetAfterId().GetValue())
	if err != nil && req.GetAfterId() != nil {
		log.Error().Err(err).Msg("invalid after id")
		return nil, status.Error(codes.InvalidArgument, handler.InvalidAfterIdError)
	}

	userID, err := uuid.Parse(req.GetUserId())
	if err != nil {
		logger.Error().Err(err).Msg("invalid user id")
		return nil, status.Error(codes.InvalidArgument, handler.InvalidUserIdError)
	}

	pageSize := req.GetPageSize().GetValue()
	if pageSize <= 0 || pageSize > handler.MaxPageSize {
		pageSize = handler.DefaultPageSize
	}

	params := db.ListUserGamesParams{
		UserID:   userID,
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

	userGames, err := h.Store.ListUserGames(ctx, params)
	if err != nil {
		logger.Error().Err(err).Msg("failed to fetch user games")
		return nil, status.Error(codes.Internal, handler.InternalServerError)
	}

	totalCount, err := h.Store.GetTotalUserGamesCount(ctx, userID)
	if err != nil {
		logger.Error().Err(err).Msg("failed to fetch user games count")
		return nil, status.Error(codes.Internal, handler.InternalServerError)
	}

	pbUserGames, err := mapUserGamesToPb(userGames)
	if err != nil {
		logger.Error().Err(err).Msg("failed to map user games to pb")
		return nil, status.Error(codes.Internal, handler.InternalServerError)
	}

	cursor := &pb.GamesCursor{
		PageSize: pageSize,
	}
	if n := len(userGames); n > 0 {
		last := userGames[n-1]
		cursor.AfterId = last.GameID.String()
		cursor.AfterEndedAt = timestamppb.New(last.EndedAt)
	}

	return &pb.ListUserGamesResponse{
		Games:      pbUserGames,
		TotalCount: totalCount,
		Cursor:     cursor,
	}, nil
}

func validateListUserGamesRequest(req *pb.ListUserGamesRequest) (violations []*errdetails.BadRequest_FieldViolation) {
	if err := protovalidate.Validate(req); err != nil {
		violations = append(violations, handler.ProtovalidateViolation(err)...)
	}

	return violations
}

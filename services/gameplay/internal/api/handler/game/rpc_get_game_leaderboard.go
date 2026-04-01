package game

import (
	"context"

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

func (h *Handler) GetGameLeaderboard(ctx context.Context, req *pb.GetGameLeaderboardRequest) (*pb.GetGameLeaderboardResponse, error) {
	violations := validateGetGameLeaderboardRequest(req)
	if violations != nil {
		return nil, handler.InvalidArgumentError(violations)
	}

	logger := log.With().Str("game_id", req.GetGameId()).Logger()

	gameID, err := uuid.Parse(req.GetGameId())
	if err != nil {
		logger.Error().Err(err).Msg("invalid game id")
		return nil, status.Error(codes.InvalidArgument, handler.InvalidGameIdError)
	}

	page := req.GetPage()
	if page < 1 {
		page = 1
	}

	offset := leaderboardOffset(page)

	entries, err := h.store.GetGameLeaderboard(ctx, db.GetGameLeaderboardParams{
		GameID:     gameID,
		PageOffset: offset,
		PageSize:   leaderboardPageSize,
	})
	if err != nil {
		logger.Error().Err(err).Msg("could not fetch game leaderboard")
		return nil, status.Error(codes.Internal, handler.InternalServerError)
	}

	totalCount, err := h.store.GetTotalGamePlayersCount(ctx, gameID)
	if err != nil {
		logger.Error().Err(err).Msg("could not fetch game leaderboard count")
		return nil, status.Error(codes.Internal, handler.InternalServerError)
	}

	players, err := mapGameLeaderboardEntriesToPb(entries)
	if err != nil {
		logger.Error().Err(err).Msg("could not map game leaderboard entries")
		return nil, status.Error(codes.Internal, handler.InternalServerError)
	}

	response := &pb.GetGameLeaderboardResponse{
		Players:    players,
		TotalCount: totalCount,
		Page:       page,
	}

	return response, nil
}

func validateGetGameLeaderboardRequest(req *pb.GetGameLeaderboardRequest) (violations []*errdetails.BadRequest_FieldViolation) {
	if err := protovalidate.Validate(req); err != nil {
		violations = append(violations, handler.ProtovalidateViolation(err)...)
	}

	return violations
}

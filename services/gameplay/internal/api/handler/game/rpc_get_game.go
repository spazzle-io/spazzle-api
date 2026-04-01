package game

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

func (h *Handler) GetGame(ctx context.Context, req *pb.GetGameRequest) (*pb.GetGameResponse, error) {
	violations := validateGetGameRequest(req)
	if violations != nil {
		return nil, handler.InvalidArgumentError(violations)
	}

	logger := log.With().Str("game_id", req.GetGameId()).Logger()

	gameID, err := uuid.Parse(req.GetGameId())
	if err != nil {
		logger.Error().Err(err).Msg("invalid game id")
		return nil, status.Error(codes.InvalidArgument, handler.InvalidGameIdError)
	}

	game, err := h.store.GetGameById(ctx, gameID)
	if err != nil {
		logger.Error().Err(err).Msg("failed to fetch game")

		if errors.Is(err, db.RecordNotFoundError) {
			return nil, status.Error(codes.NotFound, handler.GameNotFoundError)
		}
		return nil, status.Error(codes.Internal, handler.InternalServerError)
	}

	pbGame, err := mapGameToPb(game)
	if err != nil {
		logger.Error().Err(err).Msg("failed to map game to pb")
		return nil, status.Error(codes.Internal, handler.InternalServerError)
	}

	return &pb.GetGameResponse{
		Game: pbGame,
	}, nil
}

func validateGetGameRequest(req *pb.GetGameRequest) (violations []*errdetails.BadRequest_FieldViolation) {
	if err := protovalidate.Validate(req); err != nil {
		violations = append(violations, handler.ProtovalidateViolation(err)...)
	}

	return violations
}

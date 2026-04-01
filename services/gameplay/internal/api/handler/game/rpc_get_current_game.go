package game

import (
	"context"
	"errors"

	"buf.build/go/protovalidate"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/api/handler"
	pb "github.com/spazzle-io/spazzle-api/services/proto/gameplay/gameplay/v1"
	"go.temporal.io/api/serviceerror"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (h *Handler) GetCurrentGame(ctx context.Context, req *pb.GetCurrentGameRequest) (*pb.GetCurrentGameResponse, error) {
	violations := validateGetCurrentGameRequest(req)
	if violations != nil {
		return nil, handler.InvalidArgumentError(violations)
	}

	logger := log.With().Str("server_id", req.GetServerId()).Logger()

	serverID, err := uuid.Parse(req.GetServerId())
	if err != nil {
		logger.Error().Err(err).Msg("invalid server id")
		return nil, status.Error(codes.InvalidArgument, handler.InvalidServerIdError)
	}

	var cached pb.GetCurrentGameResponse
	err = h.gameCache.GetCurrentGame(ctx, serverID, &cached)
	if err == nil {
		return &cached, nil
	}

	currentGame, err := h.gfClient.GetGameState(serverID)
	if err != nil {
		logger.Warn().Err(err).Msg("failed to get game state")
		var notFound *serviceerror.NotFound
		if errors.As(err, &notFound) {
			return nil, status.Error(codes.NotFound, handler.GameNotFoundError)
		}

		return nil, status.Error(codes.Internal, handler.InternalServerError)
	}

	currentGamePb, err := mapCurrentGameToPb(currentGame)
	if err != nil {
		logger.Error().Err(err).Msg("failed to map current game state to pb")
		return nil, status.Error(codes.Internal, handler.InternalServerError)
	}

	response := &pb.GetCurrentGameResponse{
		Game: &currentGamePb,
	}

	if err := h.gameCache.SetCurrentGame(ctx, serverID, response); err != nil {
		logger.Warn().Err(err).Msg("could not cache current game")
	}

	return response, nil
}

func validateGetCurrentGameRequest(req *pb.GetCurrentGameRequest) (violations []*errdetails.BadRequest_FieldViolation) {
	if err := protovalidate.Validate(req); err != nil {
		violations = append(violations, handler.ProtovalidateViolation(err)...)
	}

	return violations
}

package game

import (
	"context"
	"strings"

	"buf.build/go/protovalidate"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/api/handler"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/eventbus"
	authPb "github.com/spazzle-io/spazzle-api/services/proto/auth/auth/v1"
	pb "github.com/spazzle-io/spazzle-api/services/proto/gameplay/gameplay/v1"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (h *Handler) ReplayGame(ctx context.Context, req *pb.ReplayGameRequest) (*pb.ReplayGameResponse, error) {
	violations := validateReplayGameRequest(req)
	if violations != nil {
		return nil, handler.InvalidArgumentError(violations)
	}

	tkPayload, err := h.authService.VerifyAccessToken(ctx, h.config.ServiceName, &authPb.VerifyAccessTokenRequest{})
	if err != nil {
		log.Error().Err(err).Msg("access token verification failed")
		return nil, status.Error(codes.Unauthenticated, handler.UnauthorizedAccessError)
	}

	logger := log.With().
		Str("user_id", tkPayload.AccessTokenPayload.UserId).
		Str("game_server_id", req.GetServerId()).
		Str("game_id", req.GetGameId()).
		Logger()

	userId, err := uuid.Parse(tkPayload.AccessTokenPayload.UserId)
	if err != nil {
		logger.Error().Err(err).Msg("invalid user id")
		return nil, status.Error(codes.Internal, handler.InternalServerError)
	}

	gameServerID, err := uuid.Parse(req.GetServerId())
	if err != nil {
		logger.Error().Err(err).Msg("invalid game server id")
		return nil, status.Error(codes.InvalidArgument, handler.InvalidServerIdError)
	}

	gameID, err := uuid.Parse(req.GetGameId())
	if err != nil {
		logger.Error().Err(err).Msg("invalid game id")
		return nil, status.Error(codes.InvalidArgument, handler.InvalidGameIdError)
	}

	// TODO: Gate replay on the payments service.
	// Only users with active commitments on the game and those with elevated permissions should be able to replay

	streamType, err := mapStreamTypeFromPb(req.GetStreamType())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, handler.InvalidStreamTypeError)
	}

	game := eventbus.GameIdentifier{
		GameServerID: gameServerID,
		GameID:       gameID,
	}

	after := strings.TrimSpace(req.GetAfter())
	if after == "" || after == "0" {
		markerID, err := h.bus.MarkerID(ctx, game, streamType, eventbus.MarkerRoundEnded)
		if err != nil {
			logger.Error().Err(err).Msg("failed to get marker round ended")
			return nil, status.Error(codes.Internal, handler.InternalServerError)
		}
		after = markerID
	}

	limit := req.GetLimit()
	if limit <= 0 {
		limit = handler.DefaultPageSize
	}

	result, err := h.bus.Replay(ctx, userId, game, streamType, after, int(limit))
	if err != nil {
		logger.Error().Err(err).Msg("failed to replay game")
		return nil, status.Error(codes.Internal, handler.InternalServerError)
	}

	messages, err := mapEventBusMessagesToPb(result.Messages)
	if err != nil {
		logger.Error().Err(err).Msg("failed to map event bus messages to pb")
		return nil, status.Error(codes.Internal, handler.InternalServerError)
	}

	response := &pb.ReplayGameResponse{
		Messages: messages,
		HasMore:  result.HasMore,
		LastId:   result.LastID,
	}

	logger.Info().Msg("game replayed successfully")

	return response, nil
}

func validateReplayGameRequest(req *pb.ReplayGameRequest) (violations []*errdetails.BadRequest_FieldViolation) {
	if err := protovalidate.Validate(req); err != nil {
		violations = append(violations, handler.ProtovalidateViolation(err)...)
	}

	return violations
}

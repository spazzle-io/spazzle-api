package game

import (
	"context"
	"strings"

	commonUtil "github.com/spazzle-io/spazzle-api/libs/common/util"

	"buf.build/go/protovalidate"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/api/handler"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/eventbus"
	pb "github.com/spazzle-io/spazzle-api/services/proto/gameplay/gameplay/v1"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	defaultReplayLimit = 500
	maxReplayLimit     = 1000
)

func (h *Handler) ReplayGame(ctx context.Context, req *pb.ReplayGameRequest) (*pb.ReplayGameResponse, error) {
	violations := validateReplayGameRequest(req)
	if violations != nil {
		return nil, handler.InvalidArgumentError(violations)
	}

	logger := log.With().
		Str("game_server_id", req.GetServerId()).
		Str("game_id", req.GetGameId()).
		Str("stream_type", req.GetStreamType().String()).
		Uint32("round", req.GetRoundNumber()).
		Logger()

	userID := uuid.Nil
	tkPayload, err := h.AuthService.VerifyAccessToken(ctx, h.Config)
	if err == nil {
		userID, err = uuid.Parse(tkPayload.AccessTokenPayload.UserId)
		if err != nil {
			logger.Error().Err(err).Msg("invalid user id")
			return nil, status.Error(codes.Internal, handler.InternalServerError)
		}
	}
	logger = logger.With().Str("user_id", userID.String()).Logger()

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

	streamType, err := mapStreamTypeFromPb(req.GetStreamType())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, handler.InvalidStreamTypeError)
	}

	game := eventbus.GameIdentifier{
		GameServerID: gameServerID,
		GameID:       gameID,
	}

	markerID := ""
	if req.GetRoundNumber() > 0 {
		roundNumber, err := commonUtil.Uint32ToUint8(req.GetRoundNumber())
		if err != nil {
			logger.Error().Err(err).Msg("invalid round number. out of range")
			return nil, status.Error(codes.InvalidArgument, handler.InvalidRoundNumberError)
		}

		markerID, err = h.Bus.MarkerID(ctx, game, streamType, eventbus.Marker{
			Round: roundNumber,
			Type:  eventbus.MarkerRoundStarted,
		})
		if err != nil {
			logger.Error().Err(err).Msg("failed to get marker round started")
			return nil, status.Error(codes.Internal, handler.InternalServerError)
		}
	}

	after := strings.TrimSpace(req.GetAfter())
	if after == "" || after == "0" {
		after = markerID
	}

	limit := req.GetLimit()
	if limit <= 0 {
		limit = defaultReplayLimit
	}
	if limit > maxReplayLimit {
		limit = maxReplayLimit
	}

	replayVisibility := eventbus.ReplayVisibilityBroadcastOnly
	if userID != uuid.Nil {
		replayVisibility = eventbus.ReplayVisibilityForClient
	}

	result, err := h.Bus.Replay(ctx, userID, game, streamType, replayVisibility, "", after, int(limit))
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

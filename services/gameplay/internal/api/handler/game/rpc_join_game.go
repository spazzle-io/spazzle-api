package game

import (
	"context"
	"errors"

	"buf.build/go/protovalidate"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/api/handler"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/api/middleware"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gamecache"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameserver"
	pb "github.com/spazzle-io/spazzle-api/services/proto/gameplay/gameplay/v1"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const retryAfterGameEndingMs = 3000

func (h *Handler) JoinGame(ctx context.Context, req *pb.JoinGameRequest) (*pb.JoinGameResponse, error) {
	violations := validateJoinGameRequest(req)
	if violations != nil {
		return nil, handler.InvalidArgumentError(violations)
	}

	logger := log.With().
		Str("server_id", req.GetServerId()).
		Str("game_role", req.GetRole().String()).
		Logger()

	serverId, err := uuid.Parse(req.GetServerId())
	if err != nil {
		logger.Error().Err(err).Msg("invalid server id")
		return nil, status.Error(codes.InvalidArgument, handler.InvalidServerIdError)
	}

	server, err := h.Store.GetServerById(ctx, serverId)
	if err != nil {
		logger.Error().Err(err).Msg("could not get server")
		return nil, handler.HandleServerDBError(err)
	}

	if server.IsArchived {
		logger.Error().Msg("server is archived")
		return nil, status.Error(codes.FailedPrecondition, handler.ServerArchivedError)
	}

	gameRole, err := mapGameRoleFromPb(req.GetRole())
	if err != nil {
		logger.Error().Err(err).Msg("could not map game role")
		return nil, status.Error(codes.InvalidArgument, handler.InvalidGameRoleError)
	}

	serverUserCtx := middleware.ServerUserContext{
		UserId: uuid.New(),
	}
	if gameRole != gameserver.Spectator {
		serverUserCtx, err = middleware.ResolveServerUserContext(
			ctx, h.Config, req.GetServerId(), h.Store, h.AuthService,
		)
		if err != nil {
			return nil, err
		}
	}
	logger = logger.With().Str("user_id", serverUserCtx.UserId.String()).Logger()

	if gameRole == gameserver.Moderator {
		if !serverUserCtx.UserServerPermissions.HasElevatedPermissions {
			logger.Error().Msg("user does not have permissions to moderate game")
			return nil, status.Error(codes.PermissionDenied, handler.UnauthorizedAccessError)
		}
	}

	gameServer, err := h.getOrCreateGameServer(serverId)
	if err != nil {
		logger.Error().Err(err).Msg("could not get or create game server")
		return nil, status.Error(codes.Internal, handler.InternalServerError)
	}

	gameState, err := gameServer.InitializeGame()
	if err != nil {
		logger.Error().Err(err).Msg("could not initialize game state")
		if errors.Is(err, gameflow.ErrGameEnding) {
			return &pb.JoinGameResponse{
				Status:       pb.JoinGameStatus_JOIN_GAME_STATUS_GAME_ENDING,
				RetryAfterMs: retryAfterGameEndingMs,
			}, nil
		}

		return nil, status.Error(codes.Internal, handler.InternalServerError)
	}

	if gameRole == gameserver.Player {
		// TODO: Call payments service and escrow stake for this game
		_ = struct{}{}
	}

	joinCodeEntry := &gamecache.JoinCodeEntry{
		UserID:   serverUserCtx.UserId,
		ServerID: serverId,
		GameID:   gameState.GameID,
		Role:     string(gameRole),
	}

	joinCode, expiresAt, err := h.GameCache.SetJoinCode(ctx, joinCodeEntry)
	if err != nil {
		logger.Error().Err(err).Msg("could not set join code")
		return nil, status.Error(codes.Internal, handler.InternalServerError)
	}

	response := &pb.JoinGameResponse{
		Status:            pb.JoinGameStatus_JOIN_GAME_STATUS_SUCCESS,
		UserId:            serverUserCtx.UserId.String(),
		GameId:            gameState.GameID.String(),
		JoinCode:          joinCode,
		JoinCodeExpiresAt: timestamppb.New(expiresAt),
	}

	logger.Info().Msg("join code issued")

	return response, nil
}

func (h *Handler) getOrCreateGameServer(serverID uuid.UUID) (*gameserver.GameServer, error) {
	gameServerConfig := &gameserver.Config{
		Env:       h.Config,
		Store:     h.Store,
		Cache:     h.Cache,
		GameCache: h.GameCache,
		Bus:       h.Bus,
		GfClient:  h.GfClient,
		WordStore: h.WordStore,
	}

	gameServer, err := h.GsManager.GetOrCreateGameServer(serverID, gameServerConfig)
	if err != nil {
		return nil, err
	}

	return gameServer, nil
}

func validateJoinGameRequest(req *pb.JoinGameRequest) (violations []*errdetails.BadRequest_FieldViolation) {
	if err := protovalidate.Validate(req); err != nil {
		violations = append(violations, handler.ProtovalidateViolation(err)...)
	}

	return violations
}

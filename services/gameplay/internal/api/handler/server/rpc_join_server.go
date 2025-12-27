package server

import (
	"context"
	"time"

	"buf.build/go/protovalidate"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/api/handler"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/api/server/websocketserver"
	authPb "github.com/spazzle-io/spazzle-api/services/proto/auth/auth/v1"
	pb "github.com/spazzle-io/spazzle-api/services/proto/gameplay/gameplay/v1"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (h *Handler) JoinServer(ctx context.Context, req *pb.JoinServerRequest) (*pb.JoinServerResponse, error) {
	violations := validateJoinServerRequest(req)
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
		Str("server_id", req.GetServerId()).
		Logger()

	userId, err := uuid.Parse(tkPayload.AccessTokenPayload.UserId)
	if err != nil {
		logger.Error().Err(err).Msg("invalid user id")
		return nil, status.Error(codes.Internal, handler.InternalServerError)
	}

	serverId, err := uuid.Parse(req.GetServerId())
	if err != nil {
		logger.Error().Err(err).Msg("invalid server id")
		return nil, status.Error(codes.InvalidArgument, handler.InvalidServerIdError)
	}

	server, err := h.store.GetServerById(ctx, serverId)
	if err != nil {
		logger.Error().Err(err).Msg("could not get server")
		return nil, HandleServerDBError(err)
	}

	// TODO: Validate that server is not archived

	joinCode, err := websocketserver.GenerateServerJoinCode(ctx, userId, serverId, h.config, h.cache)
	if err != nil {
		logger.Error().Err(err).Msg("could not generate server join code")
		return nil, status.Error(codes.Internal, handler.InternalServerError)
	}

	expiresAt := time.Now().UTC().Add(websocketserver.ServerJoinCodeCacheTTL)

	response := &pb.JoinServerResponse{
		ServerId:  server.ID.String(),
		JoinCode:  joinCode,
		ExpiresAt: timestamppb.New(expiresAt),
	}

	logger.Info().Msg("fetched server join info successfully")

	return response, nil
}

func validateJoinServerRequest(req *pb.JoinServerRequest) (violations []*errdetails.BadRequest_FieldViolation) {
	if err := protovalidate.Validate(req); err != nil {
		violations = append(violations, handler.ProtovalidateViolation(err)...)
	}

	return violations
}

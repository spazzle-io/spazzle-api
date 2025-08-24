package server

import (
	"context"
	"errors"
	"time"

	"buf.build/go/protovalidate"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog/log"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/api/handler"
	db "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/sqlc"
	authPb "github.com/spazzle-io/spazzle-api/services/proto/auth/auth/v1"
	pb "github.com/spazzle-io/spazzle-api/services/proto/gameplay/gameplay/v1"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (h *Handler) ArchiveServer(ctx context.Context, req *pb.ArchiveServerRequest) (*pb.ArchiveServerResponse, error) {
	violations := validateArchiveServerRequest(req)
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

	permissions, err := h.store.GetServerUserPermissions(ctx, db.GetServerUserPermissionsParams{
		UserID:   userId,
		ServerID: serverId,
	})
	if err != nil && !errors.Is(err, db.RecordNotFoundError) {
		logger.Error().Err(err).Msg("failed to get server permissions")
		return nil, status.Error(codes.Internal, handler.InternalServerError)
	}

	if !permissions.IsOwner {
		logger.Warn().Msg("user does not have permission to archive server")
		return nil, status.Error(codes.Unauthenticated, handler.UnauthorizedAccessError)
	}

	params := db.UpdateServerParams{
		ServerID: serverId,
		ArchivedAt: pgtype.Timestamptz{
			Time:  time.Now().UTC(),
			Valid: true,
		},
		IsArchived: pgtype.Bool{
			Bool:  true,
			Valid: true,
		},
	}

	server, err := h.store.UpdateServer(ctx, params)
	if err != nil {
		logger.Error().Err(err).Msg("failed to update server")
		return nil, HandleServerDBError(err)
	}

	pbServer, err := mapDBServerToPb(&server)
	if err != nil {
		logger.Error().Err(err).Msg("failed to map db server to pb")
		return nil, status.Error(codes.Internal, handler.InternalServerError)
	}

	response := &pb.ArchiveServerResponse{
		Server: pbServer,
	}

	logger.Info().Msg("successfully archived server")

	return response, nil
}

func validateArchiveServerRequest(req *pb.ArchiveServerRequest) (violations []*errdetails.BadRequest_FieldViolation) {
	if err := protovalidate.Validate(req); err != nil {
		violations = append(violations, handler.ProtovalidateViolation(err)...)
	}

	return violations
}

package server

import (
	"context"
	"time"

	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/api/middleware"

	"buf.build/go/protovalidate"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog/log"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/api/handler"
	db "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/sqlc"
	pb "github.com/spazzle-io/spazzle-api/services/proto/gameplay/gameplay/v1"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (h *Handler) ArchiveServer(ctx context.Context, req *pb.ArchiveServerRequest) (*pb.ArchiveServerResponse, error) {
	logger := log.With().Str("server_id", req.GetServerId()).Logger()

	violations := validateArchiveServerRequest(req)
	if violations != nil {
		return nil, handler.InvalidArgumentError(violations)
	}

	serverUserCtx, err := middleware.ResolveServerUserContext(
		ctx, h.Config, req.GetServerId(), h.Store, h.AuthService,
	)
	if err != nil {
		return nil, err
	}

	logger = logger.With().Str("user_id", serverUserCtx.UserId.String()).Logger()

	if !serverUserCtx.UserServerPermissions.IsOwner {
		logger.Warn().Msg("user does not have permission to archive server")
		return nil, status.Error(codes.Unauthenticated, handler.UnauthorizedAccessError)
	}

	params := db.UpdateServerParams{
		ServerID: serverUserCtx.ServerId,
		ArchivedAt: pgtype.Timestamptz{
			Time:  time.Now().UTC(),
			Valid: true,
		},
		IsArchived: pgtype.Bool{
			Bool:  true,
			Valid: true,
		},
	}

	server, err := h.Store.UpdateServer(ctx, params)
	if err != nil {
		logger.Error().Err(err).Msg("failed to update server")
		return nil, handler.HandleServerDBError(err)
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

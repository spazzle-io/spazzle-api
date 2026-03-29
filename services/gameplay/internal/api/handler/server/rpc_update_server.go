package server

import (
	"context"

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

func (h *Handler) UpdateServer(ctx context.Context, req *pb.UpdateServerRequest) (*pb.UpdateServerResponse, error) {
	logger := log.With().Str("server_id", req.GetServerId()).Logger()

	violations := validateUpdateServerRequest(req)
	if violations != nil {
		return nil, handler.InvalidArgumentError(violations)
	}

	serverUserCtx, err := middleware.ResolveServerUserContext(
		ctx, req.GetServerId(), h.config.ServiceName, h.store, h.authService,
	)
	if err != nil {
		return nil, err
	}

	logger = logger.With().Str("user_id", serverUserCtx.UserId.String()).Logger()

	if !serverUserCtx.UserServerPermissions.HasElevatedPermissions {
		logger.Warn().Msg("user does not have permission to update server")
		return nil, status.Error(codes.Unauthenticated, handler.UnauthorizedAccessError)
	}

	stakePerGame, err := db.ParseWeiStrToBigInt(req.GetStakePerGame().GetValue())
	if err != nil && req.GetStakePerGame() != nil {
		logger.Error().Err(err).Msg("invalid stake per game")
		return nil, status.Error(codes.InvalidArgument, handler.InvalidStakePerGameError)
	}

	params := db.UpdateServerParams{
		ServerID: serverUserCtx.ServerId,
		Name: pgtype.Text{
			String: req.GetName().GetValue(),
			Valid:  req.GetName() != nil,
		},
		IsPubliclyVisible: pgtype.Bool{
			Bool:  req.GetIsPubliclyVisible().GetValue(),
			Valid: req.GetIsPubliclyVisible() != nil,
		},
		StakePerGame: pgtype.Numeric{
			Int:   stakePerGame,
			Valid: req.GetStakePerGame() != nil,
		},
		NumRoundsPerGame: pgtype.Int4{
			Int32: req.GetNumRoundsPerGame().GetValue(),
			Valid: req.GetNumRoundsPerGame() != nil,
		},
		RoundDurationSecs: pgtype.Int4{
			Int32: req.GetRoundDurationSecs().GetValue(),
			Valid: req.GetRoundDurationSecs() != nil,
		},
		NumDrawingOptions: pgtype.Int4{
			Int32: req.GetNumDrawingOptions().GetValue(),
			Valid: req.GetNumDrawingOptions() != nil,
		},
	}

	server, err := h.store.UpdateServer(ctx, params)
	if err != nil {
		logger.Error().Err(err).Msg("failed to update server")
		return nil, handler.HandleServerDBError(err)
	}

	pbServer, err := mapDBServerToPb(&server)
	if err != nil {
		logger.Error().Err(err).Msg("failed to map db server to pb")
		return nil, status.Error(codes.Internal, handler.InternalServerError)
	}

	response := &pb.UpdateServerResponse{
		Server: pbServer,
	}

	logger.Info().Msg("server updated successfully")

	return response, nil
}

func validateUpdateServerRequest(req *pb.UpdateServerRequest) (violations []*errdetails.BadRequest_FieldViolation) {
	if err := protovalidate.Validate(req); err != nil {
		violations = append(violations, handler.ProtovalidateViolation(err)...)
	}

	return violations
}

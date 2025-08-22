package server

import (
	"context"
	"errors"

	"buf.build/go/protovalidate"

	"github.com/ethereum/go-ethereum/common"
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

func (h *Handler) CreateServer(ctx context.Context, req *pb.CreateServerRequest) (*pb.CreateServerResponse, error) {
	logger := log.With().Str("user_id", req.GetUserId()).Logger()

	violations := validateCreateServerRequest(req)
	if violations != nil {
		return nil, handler.InvalidArgumentError(violations)
	}

	_, err := h.authService.VerifyAccessToken(ctx, h.config.ServiceName, &authPb.VerifyAccessTokenRequest{
		UserId: req.GetUserId(),
	})
	if err != nil {
		logger.Error().Err(err).Msg("access token verification failed")
		return nil, status.Error(codes.Unauthenticated, handler.UnauthorizedAccessError)
	}

	userId, err := uuid.Parse(req.GetUserId())
	if err != nil {
		logger.Error().Err(err).Msg("invalid user id")
		return nil, status.Error(codes.InvalidArgument, handler.InvalidUserIdError)
	}

	stakePerGame, err := db.ParseWeiStrToBigInt(req.GetStakePerGame())
	if err != nil {
		logger.Error().Err(err).Msg("invalid stake per game")
		return nil, status.Error(codes.InvalidArgument, handler.InvalidStakePerGameError)
	}

	params := db.CreateServerParams{
		Name:              req.GetName(),
		OwnerID:           userId,
		ServerAddress:     req.GetServerAddress(),
		IsPubliclyVisible: req.GetIsPubliclyVisible(),
		StakePerGame: pgtype.Numeric{
			Int:   stakePerGame,
			Valid: true,
		},
		NumRoundsPerGame:  req.GetNumRoundsPerGame(),
		RoundDurationSecs: req.GetRoundDurationSecs(),
		NumDrawingOptions: req.GetNumDrawingOptions(),
	}
	server, err := h.store.CreateServer(ctx, params)
	if err != nil {
		logger.Error().Err(err).Msg("failed to create server")
		return nil, handleServerDBError(err)
	}

	pbServer, err := mapDBServerToPb(&server)
	if err != nil {
		logger.Error().Err(err).Msg("failed to map db server to pb")
		return nil, status.Error(codes.Internal, handler.InternalServerError)
	}

	response := &pb.CreateServerResponse{
		Server: pbServer,
	}

	logger.Info().Msg("server created successfully")

	return response, nil
}

func validateCreateServerRequest(req *pb.CreateServerRequest) (violations []*errdetails.BadRequest_FieldViolation) {
	if err := protovalidate.Validate(req); err != nil {
		violations = append(violations, handler.ProtovalidateViolation(err)...)
	}

	if isHexAddress := common.IsHexAddress(req.GetServerAddress()); !isHexAddress {
		violations = append(violations, handler.FieldViolation("serverAddress", errors.New("must be a hex address")))
	}

	return violations
}

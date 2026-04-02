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
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (h *Handler) GetUserStats(ctx context.Context, req *pb.GetUserStatsRequest) (*pb.GetUserStatsResponse, error) {
	violations := validateGetUserStatsRequest(req)
	if violations != nil {
		return nil, handler.InvalidArgumentError(violations)
	}

	logger := log.With().Str("user_id", req.GetUserId()).Logger()

	userID, err := uuid.Parse(req.GetUserId())
	if err != nil {
		logger.Error().Err(err).Msg("invalid user id")
		return nil, status.Error(codes.InvalidArgument, handler.InvalidUserIdError)
	}

	userStats, err := h.Store.GetUserStats(ctx, userID)
	if err != nil {
		logger.Error().Err(err).Msg("failed to fetch user stats")

		if errors.Is(err, db.RecordNotFoundError) {
			return nil, status.Error(codes.NotFound, handler.UserStatsNotFoundError)
		}
		return nil, status.Error(codes.Internal, handler.InternalServerError)
	}

	totalPnl, err := db.ParseDBNumericWeiToStr(userStats.TotalPnl)
	if err != nil {
		logger.Error().Err(err).Msg("failed to parse total pnl")
		return nil, status.Error(codes.Internal, handler.InternalServerError)
	}

	totalVolume, err := db.ParseDBNumericWeiToStr(userStats.TotalVolume)
	if err != nil {
		logger.Error().Err(err).Msg("failed to parse total volume")
		return nil, status.Error(codes.Internal, handler.InternalServerError)
	}

	response := &pb.GetUserStatsResponse{
		UserId:      userID.String(),
		TotalGames:  userStats.TotalGames,
		TotalScore:  userStats.TotalScore,
		TotalPnl:    totalPnl,
		TotalVolume: totalVolume,
		UpdatedAt:   timestamppb.New(userStats.UpdatedAt),
	}

	return response, nil
}

func validateGetUserStatsRequest(req *pb.GetUserStatsRequest) (violations []*errdetails.BadRequest_FieldViolation) {
	if err := protovalidate.Validate(req); err != nil {
		violations = append(violations, handler.ProtovalidateViolation(err)...)
	}

	return violations
}

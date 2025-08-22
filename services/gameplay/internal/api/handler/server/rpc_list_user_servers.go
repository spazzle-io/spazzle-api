package server

import (
	"context"

	"buf.build/go/protovalidate"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog/log"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/api/handler"
	db "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/sqlc"
	pb "github.com/spazzle-io/spazzle-api/services/proto/gameplay/gameplay/v1"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (h *Handler) ListUserServers(ctx context.Context, req *pb.ListUserServersRequest) (*pb.ListUserServersResponse, error) {
	violations := validateListUserServersRequest(req)
	if violations != nil {
		return nil, handler.InvalidArgumentError(violations)
	}

	userId, err := uuid.Parse(req.GetUserId())
	if err != nil {
		log.Error().Err(err).Msg("invalid user id")
		return nil, status.Error(codes.InvalidArgument, handler.InvalidUserIdError)
	}

	afterId, err := uuid.Parse(req.GetAfterId().GetValue())
	if err != nil && req.GetAfterId() != nil {
		log.Error().Err(err).Msg("invalid after id")
		return nil, status.Error(codes.InvalidArgument, handler.InvalidAfterIdError)
	}

	pageSize := req.GetPageSize().GetValue()
	if pageSize <= 0 || pageSize > handler.MaxPageSize {
		pageSize = handler.DefaultPageSize
	}

	params := db.ListUserServersParams{
		UserID:   userId,
		PageSize: pageSize,
		AfterID: pgtype.UUID{
			Bytes: afterId,
			Valid: req.GetAfterId() != nil,
		},
		AfterCreatedAt: pgtype.Timestamptz{
			Time:  req.GetAfterCreatedAt().AsTime(),
			Valid: req.GetAfterCreatedAt().IsValid(),
		},
	}

	userServers, err := h.store.ListUserServers(ctx, params)
	if err != nil {
		log.Error().Err(err).Msg("failed to fetch user servers")
		return nil, status.Error(codes.Internal, handler.InternalServerError)
	}

	totalCount, err := h.store.GetTotalUserServersCount(ctx, userId)
	if err != nil {
		log.Error().Err(err).Msg("failed to fetch total user server count")
		return nil, status.Error(codes.Internal, handler.InternalServerError)
	}

	pbUserServers, err := mapDBUserServersToPb(userServers)
	if err != nil {
		log.Error().Err(err).Msg("failed to map db user servers to pb")
		return nil, status.Error(codes.Internal, handler.InternalServerError)
	}

	var cursor *pb.ListServersCursor
	if n := len(userServers); n > 0 {
		last := userServers[n-1]
		cursor = &pb.ListServersCursor{
			AfterCreatedAt: timestamppb.New(last.CreatedAt),
			AfterId:        last.ID.String(),
			PageSize:       pageSize,
		}
	} else {
		cursor = &pb.ListServersCursor{PageSize: pageSize}
	}

	response := &pb.ListUserServersResponse{
		UserId:     userId.String(),
		Servers:    pbUserServers,
		TotalCount: totalCount,
		Cursor:     cursor,
	}

	log.Info().Msg("fetched user servers successfully")

	return response, nil
}

func validateListUserServersRequest(req *pb.ListUserServersRequest) (violations []*errdetails.BadRequest_FieldViolation) {
	if err := protovalidate.Validate(req); err != nil {
		violations = append(violations, handler.ProtovalidateViolation(err)...)
	}

	return violations
}

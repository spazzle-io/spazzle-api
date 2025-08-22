package server

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog/log"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/api/handler"
	db "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/sqlc"
	pb "github.com/spazzle-io/spazzle-api/services/proto/gameplay/gameplay/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (h *Handler) ListServers(ctx context.Context, req *pb.ListServersRequest) (*pb.ListServersResponse, error) {
	afterId, err := uuid.Parse(req.GetAfterId().GetValue())
	if err != nil && req.GetAfterId() != nil {
		log.Error().Err(err).Msg("invalid after id")
		return nil, status.Error(codes.InvalidArgument, handler.InvalidAfterIdError)
	}

	pageSize := req.GetPageSize().GetValue()
	if pageSize <= 0 || pageSize > handler.MaxPageSize {
		pageSize = handler.DefaultPageSize
	}

	params := db.ListServersParams{
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

	servers, err := h.store.ListServers(ctx, params)
	if err != nil {
		log.Error().Err(err).Msg("failed to fetch servers")
		return nil, status.Error(codes.Internal, handler.InternalServerError)
	}

	totalCount, err := h.store.GetTotalServerCount(ctx)
	if err != nil {
		log.Error().Err(err).Msg("failed to fetch total server count")
		return nil, status.Error(codes.Internal, handler.InternalServerError)
	}

	pbServers, err := mapDBServersToPb(servers)
	if err != nil {
		log.Error().Err(err).Msg("failed to map db servers to pb")
		return nil, status.Error(codes.Internal, handler.InternalServerError)
	}

	var cursor *pb.ListServersCursor
	if n := len(servers); n > 0 {
		last := servers[n-1]
		cursor = &pb.ListServersCursor{
			AfterCreatedAt: timestamppb.New(last.CreatedAt),
			AfterId:        last.ID.String(),
			PageSize:       pageSize,
		}
	} else {
		cursor = &pb.ListServersCursor{PageSize: pageSize}
	}

	response := &pb.ListServersResponse{
		Servers:    pbServers,
		TotalCount: totalCount,
		Cursor:     cursor,
	}

	log.Info().Msg("fetched servers successfully")

	return response, nil
}

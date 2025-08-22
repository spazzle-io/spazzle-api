package handler

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/rs/zerolog/log"
	pb "github.com/spazzle-io/spazzle-api/services/proto/users/users/v1"
	db "github.com/spazzle-io/spazzle-api/services/users/internal/db/sqlc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	defaultPageSize int32 = 30
	maxPageSize     int32 = 100
)

func (h *Handler) ListUsers(ctx context.Context, req *pb.ListUsersRequest) (*pb.ListUsersResponse, error) {
	afterId, err := uuid.Parse(req.GetAfterId())
	if err != nil && strings.TrimSpace(req.GetAfterId()) != "" {
		log.Error().Err(err).Msg("invalid after id")
		return nil, status.Error(codes.InvalidArgument, InvalidAfterIdError)
	}

	pageSize := req.GetPageSize()
	if pageSize <= 0 || pageSize > maxPageSize {
		pageSize = defaultPageSize
	}

	params := db.ListUsersParams{
		PageSize: pageSize,
		AfterID: pgtype.UUID{
			Bytes: afterId,
			Valid: strings.TrimSpace(req.GetAfterId()) != "",
		},
		AfterCreatedAt: pgtype.Timestamptz{
			Time:  req.GetAfterCreatedAt().AsTime(),
			Valid: req.GetAfterCreatedAt().IsValid(),
		},
	}

	users, err := h.store.ListUsers(ctx, params)
	if err != nil {
		log.Error().Err(err).Msg("failed to fetch users")
		return nil, status.Errorf(codes.Internal, InternalServerError)
	}

	numTotalUsers, err := h.store.GetTotalUserCount(ctx)
	if err != nil {
		log.Error().Err(err).Msg("could not get total user count")
		return nil, status.Errorf(codes.Internal, InternalServerError)
	}

	var cursor *pb.ListUsersCursor
	if n := len(users); n > 0 {
		last := users[n-1]
		cursor = &pb.ListUsersCursor{
			AfterCreatedAt: timestamppb.New(last.CreatedAt),
			AfterId:        last.ID.String(),
			PageSize:       pageSize,
		}
	}

	response := &pb.ListUsersResponse{
		Users:      mapUsers(users),
		TotalCount: numTotalUsers,
		Cursor:     cursor,
	}

	log.Info().Msg("retrieved users successfully")

	return response, nil
}

func mapUsers(dbUsers []db.User) (pbUsers []*pb.User) {
	for _, dbUser := range dbUsers {
		pbUsers = append(pbUsers, &pb.User{
			UserId:        dbUser.ID.String(),
			WalletAddress: dbUser.WalletAddress,
			GamerTag:      dbUser.GamerTag.String,
			CreatedAt:     timestamppb.New(dbUser.CreatedAt),
		})
	}

	return
}

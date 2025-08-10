package handler

import (
	"context"
	"fmt"

	"buf.build/go/protovalidate"

	"github.com/rs/zerolog/log"
	pb "github.com/spazzle-io/spazzle-api/services/proto/users/users/v1"
	db "github.com/spazzle-io/spazzle-api/services/users/internal/db/sqlc"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	defaultPageSize int32 = 30
	maxPageSize     int32 = 100
)

func (h *Handler) ListUsers(ctx context.Context, req *pb.ListUsersRequest) (*pb.ListUsersResponse, error) {
	violations := validateListUsersRequest(req)
	if violations != nil {
		return nil, invalidArgumentError(violations)
	}

	page := req.GetPage()
	if page <= 0 {
		page = 1
	}

	limit := req.GetPageSize()
	if limit <= 0 {
		limit = defaultPageSize
	}

	offset := (page - 1) * limit

	params := db.ListUsersParams{
		Limit:  limit,
		Offset: offset,
	}

	users, err := h.store.ListUsers(ctx, params)
	if err != nil {
		log.Error().Err(err).Msg("could not list users")
		return nil, status.Errorf(codes.Internal, InternalServerError)
	}

	numTotalUsers, err := h.store.GetTotalUserCount(ctx)
	if err != nil {
		log.Error().Err(err).Msg("could not get total user count")
		return nil, status.Errorf(codes.Internal, InternalServerError)
	}

	response := pb.ListUsersResponse{
		Page:          page,
		PageSize:      limit,
		NumTotalUsers: numTotalUsers,
		Users:         mapUsers(users),
	}

	log.Info().Msg("retrieved users successfully")

	return &response, nil
}

func validateListUsersRequest(req *pb.ListUsersRequest) (violations []*errdetails.BadRequest_FieldViolation) {
	if err := protovalidate.Validate(req); err != nil {
		violations = append(violations, protovalidateViolation(err)...)
	}

	if req.GetPageSize() > maxPageSize {
		violations = append(violations, fieldViolation("pageSize", fmt.Errorf("must be <= %d", maxPageSize)))
	}

	return violations
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

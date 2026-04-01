package server_admin

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

func (h *Handler) ListServerAdmins(ctx context.Context, req *pb.ListServerAdminsRequest) (*pb.ListServerAdminsResponse, error) {
	violations := validateListServerAdminRequest(req)
	if violations != nil {
		return nil, handler.InvalidArgumentError(violations)
	}

	serverId, err := uuid.Parse(req.GetServerId())
	if err != nil {
		log.Error().Err(err).Msg("invalid server id")
		return nil, status.Error(codes.InvalidArgument, handler.InvalidServerIdError)
	}

	afterUserId, err := uuid.Parse(req.GetAfterUserId().GetValue())
	if err != nil && req.GetAfterUserId() != nil {
		log.Error().Err(err).Msg("invalid after user id")
		return nil, status.Error(codes.InvalidArgument, handler.InvalidAfterIdError)
	}

	pageSize := req.GetPageSize().GetValue()
	if pageSize <= 0 || pageSize > handler.MaxPageSize {
		pageSize = handler.DefaultPageSize
	}

	params := db.ListServerAdminsParams{
		ServerID: serverId,
		PageSize: pageSize,
		AfterUserID: pgtype.UUID{
			Bytes: afterUserId,
			Valid: req.GetAfterUserId() != nil,
		},
		AfterAddedAt: pgtype.Timestamptz{
			Time:  req.GetAfterAddedAt().AsTime(),
			Valid: req.GetAfterAddedAt().IsValid(),
		},
	}

	server, err := h.store.GetServerById(ctx, serverId)
	if err != nil {
		log.Error().Err(err).Msg("failed to get server")
		return nil, handler.HandleServerDBError(err)
	}

	serverAdmins, err := h.store.ListServerAdmins(ctx, params)
	if err != nil {
		log.Error().Err(err).Msg("failed to list server admins")
		return nil, status.Error(codes.Internal, handler.InternalServerError)
	}

	pbServerAdmins := mapDBServerAdminsToPb(serverAdmins)

	cursor := &pb.ListServerAdminsCursor{
		PageSize: pageSize,
	}
	if n := len(serverAdmins); n > 0 {
		last := serverAdmins[n-1]
		cursor.AfterUserId = last.UserID.String()
		cursor.AfterAddedAt = timestamppb.New(last.AddedAt)
	}

	response := &pb.ListServerAdminsResponse{
		Admins:     pbServerAdmins,
		TotalCount: int64(server.NumAdmins),
		Cursor:     cursor,
	}

	log.Info().Msg("fetched server admins successfully")

	return response, nil
}

func validateListServerAdminRequest(req *pb.ListServerAdminsRequest) (violations []*errdetails.BadRequest_FieldViolation) {
	if err := protovalidate.Validate(req); err != nil {
		violations = append(violations, handler.ProtovalidateViolation(err)...)
	}

	return violations
}

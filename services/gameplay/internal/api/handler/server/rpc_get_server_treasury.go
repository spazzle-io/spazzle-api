package server

import (
	"context"

	"buf.build/go/protovalidate"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/api/handler"
	pb "github.com/spazzle-io/spazzle-api/services/proto/gameplay/gameplay/v1"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (h *Handler) GetServerTreasury(ctx context.Context, req *pb.GetServerTreasuryRequest) (*pb.GetServerTreasuryResponse, error) {
	logger := log.With().Str("server_id", req.GetServerId()).Logger()

	violations := validateGetServerTreasuryRequest(req)
	if violations != nil {
		return nil, handler.InvalidArgumentError(violations)
	}

	serverId, err := uuid.Parse(req.GetServerId())
	if err != nil {
		logger.Error().Err(err).Msg("invalid server id")
		return nil, status.Error(codes.InvalidArgument, handler.InvalidServerIdError)
	}

	server, err := h.Store.GetServerById(ctx, serverId)
	if err != nil {
		logger.Error().Err(err).Msg("could not get server")
		return nil, handler.HandleServerDBError(err)
	}

	treasury, err := h.Store.GetTreasury(ctx, server.ServerAddress)
	if err != nil {
		logger.Error().Err(err).Msg("could not get server treasury")
		return nil, status.Error(codes.Internal, handler.InternalServerError)
	}

	return &pb.GetServerTreasuryResponse{
		Treasury: mapDBServerTreasuryToPb(treasury),
	}, nil
}

func validateGetServerTreasuryRequest(req *pb.GetServerTreasuryRequest) (violations []*errdetails.BadRequest_FieldViolation) {
	if err := protovalidate.Validate(req); err != nil {
		violations = append(violations, handler.ProtovalidateViolation(err)...)
	}

	return violations
}

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

func (h *Handler) GetServer(ctx context.Context, req *pb.GetServerRequest) (*pb.GetServerResponse, error) {
	logger := log.With().Str("server_id", req.GetId()).Logger()

	violations := validateGetServerRequest(req)
	if violations != nil {
		return nil, handler.InvalidArgumentError(violations)
	}

	serverId, err := uuid.Parse(req.GetId())
	if err != nil {
		logger.Error().Err(err).Msg("invalid server id")
		return nil, status.Error(codes.InvalidArgument, handler.InvalidServerIdError)
	}

	server, err := h.Store.GetServerById(ctx, serverId)
	if err != nil {
		logger.Error().Err(err).Msg("could not get server")
		return nil, handler.HandleServerDBError(err)
	}

	pbServer, err := mapDBServerToPb(&server)
	if err != nil {
		logger.Error().Err(err).Msg("failed to map db server to pb")
		return nil, status.Error(codes.Internal, handler.InternalServerError)
	}

	response := &pb.GetServerResponse{
		Server: pbServer,
	}

	logger.Info().Msg("successfully retrieved server")

	return response, nil
}

func validateGetServerRequest(req *pb.GetServerRequest) (violations []*errdetails.BadRequest_FieldViolation) {
	if err := protovalidate.Validate(req); err != nil {
		violations = append(violations, handler.ProtovalidateViolation(err)...)
	}

	return violations
}

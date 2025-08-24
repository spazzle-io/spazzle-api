package server

import (
	"context"

	"buf.build/go/protovalidate"

	"github.com/rs/zerolog/log"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/api/handler"
	pb "github.com/spazzle-io/spazzle-api/services/proto/gameplay/gameplay/v1"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (h *Handler) GetServerByName(ctx context.Context, req *pb.GetServerByNameRequest) (*pb.GetServerByNameResponse, error) {
	logger := log.With().Str("server_name", req.GetName()).Logger()

	violations := validateGetServerByNameRequest(req)
	if violations != nil {
		return nil, handler.InvalidArgumentError(violations)
	}

	server, err := h.store.GetServerByName(ctx, req.GetName())
	if err != nil {
		logger.Error().Err(err).Msg("could not get server")
		return nil, HandleServerDBError(err)
	}

	pbServer, err := mapDBServerToPb(&server)
	if err != nil {
		logger.Error().Err(err).Msg("failed to map db server to pb")
		return nil, status.Error(codes.Internal, handler.InternalServerError)
	}

	response := &pb.GetServerByNameResponse{
		Server: pbServer,
	}

	logger.Info().Msg("successfully retrieved server by name")

	return response, nil
}

func validateGetServerByNameRequest(req *pb.GetServerByNameRequest) (violations []*errdetails.BadRequest_FieldViolation) {
	if err := protovalidate.Validate(req); err != nil {
		violations = append(violations, handler.ProtovalidateViolation(err)...)
	}

	return violations
}

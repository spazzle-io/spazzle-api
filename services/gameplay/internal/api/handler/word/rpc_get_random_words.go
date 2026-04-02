package word

import (
	"context"
	"errors"

	"buf.build/go/protovalidate"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/api/handler"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/wordstore"
	pb "github.com/spazzle-io/spazzle-api/services/proto/gameplay/gameplay/v1"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const defaultRandomWordsLimit = 3

func (h *Handler) GetRandomWords(ctx context.Context, req *pb.GetRandomWordsRequest) (*pb.GetRandomWordsResponse, error) {
	logger := log.With().Str("server_id", req.GetServerId()).Logger()

	violations := validateGetRandomWordsRequest(req)
	if violations != nil {
		return nil, handler.InvalidArgumentError(violations)
	}

	serverId, err := uuid.Parse(req.GetServerId())
	if err != nil {
		logger.Error().Err(err).Msg("invalid server id")
		return nil, status.Error(codes.InvalidArgument, handler.InvalidServerIdError)
	}

	limit := req.GetLimit().GetValue()
	if limit <= 0 {
		limit = defaultRandomWordsLimit
	}

	randomWords, err := h.WordStore.GetRandomWords(ctx, h.Store, serverId, int(limit))
	if err != nil {
		logger.Error().Err(err).Msg("failed to get random words")

		if errors.Is(err, wordstore.ErrServerNotfound) {
			return nil, status.Error(codes.NotFound, handler.ServerNotFoundError)
		}
		return nil, status.Error(codes.Internal, handler.InternalServerError)
	}

	response := &pb.GetRandomWordsResponse{
		Words: mapWordStoreWordsToPb(randomWords),
	}

	logger.Info().Msg("successfully fetched server random words")

	return response, nil
}

func validateGetRandomWordsRequest(req *pb.GetRandomWordsRequest) (violations []*errdetails.BadRequest_FieldViolation) {
	if err := protovalidate.Validate(req); err != nil {
		violations = append(violations, handler.ProtovalidateViolation(err)...)
	}

	return violations
}

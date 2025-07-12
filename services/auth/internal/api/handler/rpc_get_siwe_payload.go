package handler

import (
	"context"
	"errors"

	"buf.build/go/protovalidate"

	"github.com/ethereum/go-ethereum/common"
	"github.com/rs/zerolog/log"
	"github.com/spazzle-io/spazzle-api/services/auth/internal/siwe"
	pb "github.com/spazzle-io/spazzle-api/services/proto/auth/auth/v1"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (h *Handler) GetSIWEPayload(
	ctx context.Context,
	req *pb.GetSIWEPayloadRequest,
) (*pb.GetSIWEPayloadResponse, error) {
	logger := log.With().Str("wallet_address", req.GetWalletAddress()).Logger()

	violations := validateGetSIWEPayloadRequest(req)
	if violations != nil {
		return nil, invalidArgumentError(violations)
	}

	payload, err := siwe.GenerateSIWEPayload(
		ctx, h.config, h.cache, req.GetDomain(), req.GetUri(), req.GetChainId(), req.GetWalletAddress(),
	)
	if err != nil {
		logger.Error().Err(err).Msg("could not generate SIWE payload")

		if errors.Is(err, siwe.ErrInvalidInput) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		return nil, status.Error(codes.Internal, InternalServerError)
	}

	response := &pb.GetSIWEPayloadResponse{
		Message:       payload.Message,
		Nonce:         payload.Nonce,
		WalletAddress: payload.WalletAddress,
		IssuedAt:      timestamppb.New(payload.IssuedAt),
		ExpiresAt:     timestamppb.New(payload.ExpiresAt),
	}

	logger.Info().Msg("SIWE payload generated successfully")

	return response, nil
}

func validateGetSIWEPayloadRequest(req *pb.GetSIWEPayloadRequest) (violations []*errdetails.BadRequest_FieldViolation) {
	if err := protovalidate.Validate(req); err != nil {
		violations = append(violations, protovalidateViolation(err)...)
	}

	if isHexAddress := common.IsHexAddress(req.GetWalletAddress()); !isHexAddress {
		violations = append(violations, fieldViolation("walletAddress", errors.New("must be a hex address")))
	}

	return violations
}

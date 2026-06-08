package handler

import (
	"context"
	"errors"

	"buf.build/go/protovalidate"

	"github.com/ethereum/go-ethereum/common"
	"github.com/rs/zerolog/log"
	pb "github.com/spazzle-io/spazzle-api/services/proto/users/users/v1"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (h *Handler) GetUserByWalletAddress(ctx context.Context, req *pb.GetUserByWalletAddressRequest) (*pb.GetUserByWalletAddressResponse, error) {
	logger := log.With().Str("wallet_address", req.GetWalletAddress()).Logger()

	violations := validateGetUserByWalletAddressRequest(req)
	if violations != nil {
		return nil, invalidArgumentError(violations)
	}

	user, err := h.Store.GetUserByWalletAddress(ctx, req.GetWalletAddress())
	if err != nil {
		logger.Error().Err(err).Msg("could not get user")
		return nil, status.Error(codes.NotFound, UserNotFoundError)
	}

	logger = logger.With().Str("user_id", user.ID.String()).Logger()

	response := &pb.GetUserByWalletAddressResponse{
		User: &pb.User{
			Id:            user.ID.String(),
			WalletAddress: user.WalletAddress,
			GamerTag:      user.GamerTag.String,
			CreatedAt:     timestamppb.New(user.CreatedAt),
		},
	}

	logger.Info().Msg("successfully retrieved user")

	return response, nil
}

func validateGetUserByWalletAddressRequest(req *pb.GetUserByWalletAddressRequest) (violations []*errdetails.BadRequest_FieldViolation) {
	if err := protovalidate.Validate(req); err != nil {
		violations = append(violations, protovalidateViolation(err)...)
	}

	if isHexAddress := common.IsHexAddress(req.GetWalletAddress()); !isHexAddress {
		violations = append(violations, fieldViolation("walletAddress", errors.New("must be a hex address")))
	}

	return violations
}

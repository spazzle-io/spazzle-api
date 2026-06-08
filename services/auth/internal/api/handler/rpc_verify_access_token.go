package handler

import (
	"context"

	"github.com/rs/zerolog/log"
	"github.com/spazzle-io/spazzle-api/services/auth/internal/api/middleware"
	"github.com/spazzle-io/spazzle-api/services/auth/internal/token"
	pb "github.com/spazzle-io/spazzle-api/services/proto/auth/auth/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (h *Handler) VerifyAccessToken(
	ctx context.Context,
	_ *pb.VerifyAccessTokenRequest,
) (*pb.VerifyAccessTokenResponse, error) {
	tkPayload, err := middleware.AuthorizeToken(ctx, h.TokenMaker, token.AccessToken, nil)
	if err != nil {
		log.Error().Err(err).Msg("could not authorize token")
		return nil, status.Error(codes.Unauthenticated, UnauthorizedAccessError)
	}

	logger := log.With().Str("user_id", tkPayload.UserID.String()).Logger()

	resPayloadRole := pb.AccessTokenPayload_ROLE_UNSPECIFIED
	switch tkPayload.Role {
	case token.User:
		resPayloadRole = pb.AccessTokenPayload_ROLE_USER
	case token.Admin:
		resPayloadRole = pb.AccessTokenPayload_ROLE_ADMIN
	}

	res := &pb.VerifyAccessTokenResponse{
		AccessTokenPayload: &pb.AccessTokenPayload{
			Id:            tkPayload.ID.String(),
			UserId:        tkPayload.UserID.String(),
			WalletAddress: tkPayload.WalletAddress,
			Role:          resPayloadRole,
			IssuedAt:      timestamppb.New(tkPayload.IssuedAt),
			ExpiresAt:     timestamppb.New(tkPayload.ExpiresAt),
		},
	}

	logger.Info().Msg("access token verified")

	return res, nil
}

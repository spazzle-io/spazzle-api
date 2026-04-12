package handler

import (
	"context"
	"errors"

	"buf.build/go/protovalidate"

	"github.com/ethereum/go-ethereum/common"
	"github.com/rs/zerolog"
	db "github.com/spazzle-io/spazzle-api/services/users/internal/db/sqlc"

	"github.com/rs/zerolog/log"
	authPb "github.com/spazzle-io/spazzle-api/services/proto/auth/auth/v1"
	pb "github.com/spazzle-io/spazzle-api/services/proto/users/users/v1"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	UserAlreadyExists string = "User already exists"
	GamerTagInUse     string = "Gamer tag already in use"
)

func (h *Handler) AuthenticateUser(ctx context.Context, req *pb.AuthenticateUserRequest) (*pb.AuthenticateUserResponse, error) {
	logger := log.With().Str("wallet_address", req.GetWalletAddress()).Logger()

	violations := validateAuthenticateUserRequest(req)
	if violations != nil {
		return nil, invalidArgumentError(violations)
	}

	user, authenticateRes, err := h.handleAuthenticateUser(ctx, req, logger)
	if err != nil {
		return nil, err
	}

	response := &pb.AuthenticateUserResponse{
		User: &pb.User{
			Id:            user.ID.String(),
			WalletAddress: user.WalletAddress,
			GamerTag:      user.GamerTag.String,
			CreatedAt:     timestamppb.New(user.CreatedAt),
		},
		Session: &pb.Session{
			SessionId:             authenticateRes.Session.SessionId,
			AccessToken:           authenticateRes.Session.AccessToken,
			RefreshToken:          authenticateRes.Session.RefreshToken,
			AccessTokenExpiresAt:  authenticateRes.Session.AccessTokenExpiresAt,
			RefreshTokenExpiresAt: authenticateRes.Session.RefreshTokenExpiresAt,
			TokenType:             authenticateRes.Session.TokenType,
		},
	}

	logger.Info().Msg("user authenticated successfully")

	return response, nil
}

func (h *Handler) handleAuthenticateUser(
	ctx context.Context,
	req *pb.AuthenticateUserRequest,
	logger zerolog.Logger,
) (*db.User, *authPb.AuthenticateResponse, error) {
	user, err := h.store.GetUserByWalletAddress(ctx, req.GetWalletAddress())
	if err != nil && !errors.Is(err, db.RecordNotFoundError) {
		logger.Error().Err(err).Msg("could not fetch user by wallet address")
		return nil, nil, status.Error(codes.Internal, InternalServerError)
	}

	if user != (db.User{}) {
		authenticateRes, err := h.handleExistingUser(ctx, &user, req.GetSignature(), logger)
		return &user, authenticateRes, err
	}

	return h.handleCreateUser(ctx, req.GetWalletAddress(), req.GetSignature(), logger)
}

func (h *Handler) handleExistingUser(
	ctx context.Context,
	user *db.User,
	signature string,
	logger zerolog.Logger,
) (*authPb.AuthenticateResponse, error) {
	authenticateRequest := &authPb.AuthenticateRequest{
		WalletAddress: user.WalletAddress,
		UserId:        user.ID.String(),
		Signature:     signature,
	}

	authenticateRes, err := h.authService.Authenticate(ctx, h.config, authenticateRequest)
	if err != nil {
		logger.Error().Err(err).Msg("could not authenticate user")
		return nil, status.Error(codes.Internal, InternalServerError)
	}

	return authenticateRes, nil
}

func (h *Handler) handleCreateUser(
	ctx context.Context,
	walletAddress string,
	signature string,
	logger zerolog.Logger,
) (*db.User, *authPb.AuthenticateResponse, error) {
	var err error
	authenticateRes := &authPb.AuthenticateResponse{}

	params := db.CreateUserTxParams{
		WalletAddress: walletAddress,
		AfterCreate: func(user db.User) error {
			authenticateRequest := &authPb.AuthenticateRequest{
				WalletAddress: walletAddress,
				UserId:        user.ID.String(),
				Signature:     signature,
			}

			authenticateRes, err = h.authService.Authenticate(ctx, h.config, authenticateRequest)
			if err != nil {
				logger.Error().Err(err).Msg("could not authenticate user")
			}

			return err
		},
	}

	txResult, err := h.store.CreateUserTx(ctx, params)
	if err != nil {
		logger.Error().Err(err).Msg("could not create user")
		return nil, nil, handleCreateUserTxError(err)
	}

	return &txResult.User, authenticateRes, nil
}

func handleCreateUserTxError(err error) error {
	switch {
	case errors.Is(err, db.ErrUserAlreadyExists):
		return status.Error(codes.AlreadyExists, UserAlreadyExists)
	case errors.Is(err, db.ErrGamerTagAlreadyInUse):
		return status.Error(codes.AlreadyExists, GamerTagInUse)
	}

	return status.Error(codes.Internal, InternalServerError)
}

func validateAuthenticateUserRequest(req *pb.AuthenticateUserRequest) (violations []*errdetails.BadRequest_FieldViolation) {
	if err := protovalidate.Validate(req); err != nil {
		violations = append(violations, protovalidateViolation(err)...)
	}

	if isHexAddress := common.IsHexAddress(req.GetWalletAddress()); !isHexAddress {
		violations = append(violations, fieldViolation("walletAddress", errors.New("must be a hex address")))
	}

	return violations
}

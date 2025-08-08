package handler

import (
	"context"
	"errors"
	"strings"

	"buf.build/go/protovalidate"

	"github.com/ethereum/go-ethereum/common"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog/log"
	authPb "github.com/spazzle-io/spazzle-api/services/proto/auth/auth/v1"
	pb "github.com/spazzle-io/spazzle-api/services/proto/users/users/v1"
	db "github.com/spazzle-io/spazzle-api/services/users/internal/db/sqlc"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	UserAlreadyExists string = "User already exists"
	GamerTagInUse     string = "Gamer tag already in use"
)

func (h *Handler) CreateUser(ctx context.Context, req *pb.CreateUserRequest) (*pb.CreateUserResponse, error) {
	var err error
	authenticateResponse := &authPb.AuthenticateResponse{}

	logger := log.With().Str("wallet_address", req.GetWalletAddress()).Logger()

	violations := validateCreateUserRequest(req)
	if violations != nil {
		return nil, invalidArgumentError(violations)
	}

	params := db.CreateUserTxParams{
		CreateUserParams: db.CreateUserParams{
			WalletAddress: req.GetWalletAddress(),
			GamerTag: pgtype.Text{
				String: req.GetGamerTag(),
				Valid:  len(strings.TrimSpace(req.GetGamerTag())) > 0,
			},
		},
		AfterCreate: func(user db.User) error {
			authenticateRequest := &authPb.AuthenticateRequest{
				WalletAddress: req.GetWalletAddress(),
				UserId:        user.ID.String(),
				Signature:     req.GetSignature(),
			}

			authenticateResponse, err = h.authService.Authenticate(ctx, h.config.ServiceName, authenticateRequest)
			if err != nil {
				logger.Error().Err(err).Msg("could not authenticate user")
			}

			return err
		},
	}

	txResult, err := h.store.CreateUserTx(ctx, params)
	if err != nil {
		logger.Error().Err(err).Msg("could not create user")
		return nil, handleCreateUserTxError(err)
	}

	response := &pb.CreateUserResponse{
		User: &pb.User{
			UserId:        txResult.User.ID.String(),
			WalletAddress: txResult.User.WalletAddress,
			GamerTag:      txResult.User.GamerTag.String,
			CreatedAt:     timestamppb.New(txResult.User.CreatedAt),
		},
		Session: &pb.Session{
			SessionId:             authenticateResponse.Session.SessionId,
			AccessToken:           authenticateResponse.Session.AccessToken,
			RefreshToken:          authenticateResponse.Session.RefreshToken,
			AccessTokenExpiresAt:  authenticateResponse.Session.AccessTokenExpiresAt,
			RefreshTokenExpiresAt: authenticateResponse.Session.RefreshTokenExpiresAt,
			TokenType:             authenticateResponse.Session.TokenType,
		},
	}

	logger.Info().Msg("user created successfully")

	return response, nil
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

func validateCreateUserRequest(req *pb.CreateUserRequest) (violations []*errdetails.BadRequest_FieldViolation) {
	if err := protovalidate.Validate(req); err != nil {
		violations = append(violations, protovalidateViolation(err)...)
	}

	if isHexAddress := common.IsHexAddress(req.GetWalletAddress()); !isHexAddress {
		violations = append(violations, fieldViolation("walletAddress", errors.New("must be a hex address")))
	}

	return violations
}

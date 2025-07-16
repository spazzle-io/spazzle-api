package handler

import (
	"context"
	"errors"

	"github.com/rs/zerolog"

	"buf.build/go/protovalidate"

	"github.com/ethereum/go-ethereum/common"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	commonMiddleware "github.com/spazzle-io/spazzle-api/libs/common/middleware"
	commonUtil "github.com/spazzle-io/spazzle-api/libs/common/util"
	db "github.com/spazzle-io/spazzle-api/services/auth/internal/db/sqlc"
	"github.com/spazzle-io/spazzle-api/services/auth/internal/siwe"
	"github.com/spazzle-io/spazzle-api/services/auth/internal/token"
	"github.com/spazzle-io/spazzle-api/services/auth/internal/util"
	pb "github.com/spazzle-io/spazzle-api/services/proto/auth/auth/v1"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var allowedServices = []commonMiddleware.Service{util.Users}

func (h *Handler) Authenticate(ctx context.Context, req *pb.AuthenticateRequest) (*pb.AuthenticateResponse, error) {
	logger := log.With().Str("wallet_address", req.GetWalletAddress()).Logger()

	violations := validateAuthenticateRequest(req)
	if violations != nil {
		return nil, invalidArgumentError(violations)
	}

	userId, err := uuid.Parse(req.GetUserId())
	if err != nil {
		logger.Error().Err(err).Str("user_id", req.GetUserId()).Msg("could not parse user id")
		return nil, status.Error(codes.InvalidArgument, InvalidUserIdError)
	}

	logger = log.With().Str("user_id", userId.String()).Logger()

	authenticatedService, err := commonMiddleware.AuthorizeService(ctx, allowedServices)
	if err != nil {
		log.Error().Err(err).Msg("failed to authorize service")
		return nil, status.Error(codes.PermissionDenied, UnauthorizedAccessError)
	}

	logger = log.With().Str("authenticated_service", string(authenticatedService)).Logger()

	cachedSIWEMessage, err := siwe.FetchSIWEMessage(ctx, h.config, h.cache, req.GetWalletAddress())
	if err != nil {
		logger.Error().Err(err).Msg("could not fetch cached SIWE message")

		if errors.Is(err, siwe.ErrInvalidInput) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		return nil, status.Error(codes.Internal, InternalServerError)
	}

	isSignatureValid, err := commonUtil.IsEthereumSignatureValid(req.GetWalletAddress(), cachedSIWEMessage, req.GetSignature())
	if err != nil || !isSignatureValid {
		logger.Error().Err(err).Msg("SIWE signature not valid")
		return nil, status.Error(codes.InvalidArgument, SignatureVerificationError)
	}

	credential, session, err := h.handleCredentialAndSession(ctx, userId, req, logger)
	if err != nil {
		return nil, err
	}

	response := &pb.AuthenticateResponse{
		Credential: &pb.Credential{
			Id:            credential.ID.String(),
			UserId:        credential.UserID.String(),
			WalletAddress: credential.WalletAddress,
			CreatedAt:     timestamppb.New(credential.CreatedAt),
		},
		Session: &pb.Session{
			SessionId:             session.ID,
			AccessToken:           session.AccessToken,
			RefreshToken:          session.RefreshToken,
			AccessTokenExpiresAt:  timestamppb.New(session.AccessTokenPayload.ExpiresAt),
			RefreshTokenExpiresAt: timestamppb.New(session.RefreshTokenPayload.ExpiresAt),
			TokenType:             authorizationBearer,
		},
	}

	logger.Info().Str("credential_id", credential.ID.String()).Msg("authenticated successfully")

	return response, nil
}

func (h *Handler) handleCredentialAndSession(
	ctx context.Context,
	userId uuid.UUID,
	req *pb.AuthenticateRequest,
	logger zerolog.Logger,
) (db.Credential, *Session, error) {
	credential, err := h.store.GetCredentialByWalletAddress(ctx, req.GetWalletAddress())
	if err != nil && !errors.Is(err, db.RecordNotFoundError) {
		logger.Error().Err(err).Msg("could not fetch credential by wallet address")
		return db.Credential{}, nil, status.Error(codes.Internal, InternalServerError)
	}

	if credential != (db.Credential{}) {
		session, err := h.handleExistingCredential(ctx, credential, userId, logger)
		return credential, session, err
	}

	return h.createCredentialAndSession(ctx, userId, req.GetWalletAddress(), logger)
}

func (h *Handler) handleExistingCredential(
	ctx context.Context,
	credential db.Credential,
	userId uuid.UUID,
	logger zerolog.Logger,
) (*Session, error) {
	if credential.UserID != userId {
		logger.Error().
			Str("credential_user_id", credential.UserID.String()).
			Msg("provided user id does not match credential")
		return nil, status.Error(codes.PermissionDenied, UnauthorizedAccessError)
	}

	session, err := NewSession(
		ctx, credential.UserID, credential.WalletAddress, token.Role(credential.Role), h.config, h.tokenMaker, h.store,
	)
	if err != nil {
		logger.Error().Err(err).Msg("could not create session")
		return nil, status.Error(codes.Internal, InternalServerError)
	}

	return session, nil
}

func (h *Handler) createCredentialAndSession(
	ctx context.Context,
	userId uuid.UUID,
	walletAddress string,
	logger zerolog.Logger,
) (db.Credential, *Session, error) {
	var session *Session

	txRes, err := h.store.CreateCredentialTx(ctx, db.CreateCredentialTxParams{
		CreateCredentialParams: db.CreateCredentialParams{
			WalletAddress: walletAddress,
			UserID:        userId,
		},
		AfterCreate: func(credential db.Credential) error {
			s, err := NewSession(
				ctx, credential.UserID, credential.WalletAddress, token.Role(credential.Role), h.config, h.tokenMaker, h.store,
			)
			if err != nil {
				logger.Error().Err(err).Msg("could not create session")
				return status.Error(codes.Internal, InternalServerError)
			}

			session = s
			return nil
		},
	})
	if err != nil {
		logger.Error().Err(err).Msg("could not create credential")
		return db.Credential{}, nil, status.Error(codes.Internal, InternalServerError)
	}

	return txRes.Credential, session, nil
}

func validateAuthenticateRequest(req *pb.AuthenticateRequest) (violations []*errdetails.BadRequest_FieldViolation) {
	if err := protovalidate.Validate(req); err != nil {
		violations = append(violations, protovalidateViolation(err)...)
	}

	if isHexAddress := common.IsHexAddress(req.GetWalletAddress()); !isHexAddress {
		violations = append(violations, fieldViolation("walletAddress", errors.New("must be a hex address")))
	}

	return violations
}

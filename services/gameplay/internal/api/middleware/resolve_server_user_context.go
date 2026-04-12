package middleware

import (
	"context"
	"errors"

	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/util"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/api/handler"
	db "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/sqlc"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/services"
	authPb "github.com/spazzle-io/spazzle-api/services/proto/auth/auth/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ServerUserContext struct {
	ServerId              uuid.UUID
	UserId                uuid.UUID
	AccessTokenPayload    *authPb.AccessTokenPayload
	UserServerPermissions db.GetServerUserPermissionsRow
}

func ResolveServerUserContext(
	ctx context.Context,
	config *util.Config,
	serverId string,
	store db.Store,
	authService services.AuthGrpcService,
) (serverUserContext ServerUserContext, err error) {
	logger := log.With().Str("server_id", serverId).Logger()

	serverUserContext.ServerId, err = uuid.Parse(serverId)
	if err != nil {
		logger.Error().Err(err).Msg("invalid server id")
		return serverUserContext, status.Error(codes.InvalidArgument, handler.InvalidServerIdError)
	}

	verifyAccessTokenResp, err := authService.VerifyAccessToken(ctx, config, &authPb.VerifyAccessTokenRequest{})
	if err != nil {
		logger.Error().Err(err).Msg("access token verification failed")
		return serverUserContext, status.Error(codes.Unauthenticated, handler.UnauthorizedAccessError)
	}

	serverUserContext.AccessTokenPayload = verifyAccessTokenResp.GetAccessTokenPayload()

	serverUserContext.UserId, err = uuid.Parse(serverUserContext.AccessTokenPayload.UserId)
	if err != nil {
		logger.Error().Err(err).Msg("invalid user id")
		return serverUserContext, status.Error(codes.Internal, handler.InternalServerError)
	}

	logger = logger.With().Str("user_id", serverUserContext.UserId.String()).Logger()

	serverUserContext.UserServerPermissions, err = store.GetServerUserPermissions(ctx, db.GetServerUserPermissionsParams{
		UserID:   serverUserContext.UserId,
		ServerID: serverUserContext.ServerId,
	})
	if err != nil && !errors.Is(err, db.RecordNotFoundError) {
		logger.Error().Err(err).Msg("failed to get server permissions")
		return serverUserContext, status.Error(codes.Internal, handler.InternalServerError)
	}

	return
}

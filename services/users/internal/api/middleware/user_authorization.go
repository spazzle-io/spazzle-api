package middleware

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	pb "github.com/spazzle-io/spazzle-api/services/proto/auth/auth/v1"
	"github.com/spazzle-io/spazzle-api/services/users/internal/services"
	"github.com/spazzle-io/spazzle-api/services/users/internal/util"
)

func AuthorizeUser(ctx context.Context, userId uuid.UUID, config util.Config, authService services.AuthGrpcService) (*pb.AccessTokenPayload, error) {
	payload := &pb.VerifyAccessTokenRequest{
		UserId: userId.String(),
	}

	response, err := authService.VerifyAccessToken(ctx, config.ServiceName, payload)
	if err != nil {
		return nil, fmt.Errorf("could not verify access token: %w", err)
	}

	return response.GetAccessTokenPayload(), nil
}

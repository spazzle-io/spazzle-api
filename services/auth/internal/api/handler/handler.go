package handler

import (
	"time"

	"github.com/spazzle-io/spazzle-api/services/auth/internal/infra"

	pb "github.com/spazzle-io/spazzle-api/services/proto/auth/auth/v1"

	commonMiddleware "github.com/spazzle-io/spazzle-api/libs/common/middleware"
)

type Handler struct {
	pb.UnimplementedAuthServiceServer

	*infra.Resources
}

func New(res *infra.Resources) *Handler {
	return &Handler{
		Resources: res,
	}
}

func (h *Handler) RateLimits() map[string]commonMiddleware.Rate {
	return map[string]commonMiddleware.Rate{
		"/auth.v1.AuthService/GetSIWEPayload":      {Aliases: []string{"GET:/auth/siwe-payload"}, Limit: 10, Period: time.Minute, Identifier: "GetSIWEPayload"},
		"/auth.v1.AuthService/Authenticate":        {Aliases: []string{"POST:/auth/authenticate"}, Limit: 10, Period: time.Minute, Identifier: "Authenticate"},
		"/auth.v1.AuthService/VerifyAccessToken":   {Aliases: []string{"POST:/auth/verify-access-token"}, Limit: 120, Period: time.Minute, Identifier: "VerifyAccessToken"},
		"/auth.v1.AuthService/RefreshAccessToken":  {Aliases: []string{"POST:/auth/refresh-access-token"}, Limit: 10, Period: time.Minute, Identifier: "RefreshAccessToken"},
		"/auth.v1.AuthService/RevokeRefreshTokens": {Aliases: []string{"POST:/auth/revoke-refresh-tokens"}, Limit: 10, Period: time.Minute, Identifier: "RevokeRefreshTokens"},
	}
}

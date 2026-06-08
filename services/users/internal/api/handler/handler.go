package handler

import (
	"time"

	"github.com/spazzle-io/spazzle-api/services/users/internal/infra"

	commonMiddleware "github.com/spazzle-io/spazzle-api/libs/common/middleware"
	pb "github.com/spazzle-io/spazzle-api/services/proto/users/users/v1"
)

type Handler struct {
	pb.UnimplementedUserServiceServer

	*infra.Resources
}

func New(res *infra.Resources) *Handler {
	return &Handler{
		Resources: res,
	}
}

func (h *Handler) RateLimits() map[string]commonMiddleware.Rate {
	return map[string]commonMiddleware.Rate{
		"/users.v1.UsersService/AuthenticateUser":       {Aliases: []string{"POST:/users/authenticate"}, Limit: 10, Period: time.Minute, Identifier: "AuthenticateUser"},
		"/users.v1.UsersService/GetUser":                {Aliases: []string{"GET:/users/{uuid}"}, Limit: 120, Period: time.Minute, Identifier: "GetUser"},
		"/users.v1.UsersService/GetUserByWalletAddress": {Aliases: []string{"GET:/users/by-wallet-address/{evm_address}"}, Limit: 120, Period: time.Minute, Identifier: "GetUserByWalletAddress"},
		"/users.v1.UsersService/ListUsers":              {Aliases: []string{"GET:/users"}, Limit: 120, Period: time.Minute, Identifier: "ListUsers"},
		"/users.v1.UsersService/UpdateUser":             {Aliases: []string{"PUT:/users/{uuid}"}, Limit: 30, Period: time.Minute, Identifier: "UpdateUser"},
	}
}

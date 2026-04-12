package handler

import (
	"time"

	commonCache "github.com/spazzle-io/spazzle-api/libs/common/cache"
	commonMiddleware "github.com/spazzle-io/spazzle-api/libs/common/middleware"
	pb "github.com/spazzle-io/spazzle-api/services/proto/users/users/v1"
	db "github.com/spazzle-io/spazzle-api/services/users/internal/db/sqlc"
	"github.com/spazzle-io/spazzle-api/services/users/internal/services"
	"github.com/spazzle-io/spazzle-api/services/users/internal/util"
)

type Handler struct {
	pb.UnimplementedUserServiceServer

	config      *util.Config
	store       db.Store
	cache       commonCache.Cache
	authService services.AuthGrpcService
}

func New(config *util.Config, store db.Store, cache commonCache.Cache, authService services.AuthGrpcService) *Handler {
	return &Handler{
		config:      config,
		store:       store,
		cache:       cache,
		authService: authService,
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

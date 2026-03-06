package word

import (
	"time"

	commonCache "github.com/spazzle-io/spazzle-api/libs/common/cache"
	commonMiddleware "github.com/spazzle-io/spazzle-api/libs/common/middleware"
	db "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/sqlc"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/services"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/util"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/wordstore"
	pb "github.com/spazzle-io/spazzle-api/services/proto/gameplay/gameplay/v1"
)

type HandlerConfig struct {
	Config      util.Config
	Store       db.Store
	Cache       commonCache.Cache
	WordStore   wordstore.Store
	AuthService services.AuthGrpcService
}

type Handler struct {
	pb.UnimplementedWordServiceServer

	config      util.Config
	store       db.Store
	cache       commonCache.Cache
	authService services.AuthGrpcService
	wordStore   wordstore.Store
}

func New(cfg *HandlerConfig) *Handler {
	return &Handler{
		config:      cfg.Config,
		store:       cfg.Store,
		cache:       cfg.Cache,
		wordStore:   cfg.WordStore,
		authService: cfg.AuthService,
	}
}

func (h *Handler) RateLimits() map[string]commonMiddleware.Rate {
	return map[string]commonMiddleware.Rate{
		"/gameplay.v1.WordService/AddWords":       {Aliases: []string{"POST:/servers/{uuid}/words"}, Limit: 30, Period: time.Minute, Identifier: "AddWords"},
		"/gameplay.v1.WordService/GetRandomWords": {Aliases: []string{"GET:/servers/{uuid}/words:random"}, Limit: 60, Period: time.Minute, Identifier: "GetRandomWords"},
		"/gameplay.v1.WordService/ListWords":      {Aliases: []string{"GET:/servers/{uuid}/words"}, Limit: 60, Period: time.Minute, Identifier: "ListWords"},
		"/gameplay.v1.WordService/RemoveWords":    {Aliases: []string{"POST:/servers/{uuid}/words:remove"}, Limit: 30, Period: time.Minute, Identifier: "RemoveWords"},
		"/gameplay.v1.WordService/RemoveAllWords": {Aliases: []string{"DELETE:/servers/{uuid}/words"}, Limit: 30, Period: time.Minute, Identifier: "RemoveAllWords"},
	}
}

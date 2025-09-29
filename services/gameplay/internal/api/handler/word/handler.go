package word

import (
	"time"

	"github.com/rs/zerolog/log"
	commonCache "github.com/spazzle-io/spazzle-api/libs/common/cache"
	commonMiddleware "github.com/spazzle-io/spazzle-api/libs/common/middleware"
	db "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/sqlc"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/services"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/util"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/wordstore"
	pb "github.com/spazzle-io/spazzle-api/services/proto/gameplay/gameplay/v1"
)

type Handler struct {
	pb.UnimplementedWordServiceServer

	config      util.Config
	store       db.Store
	cache       commonCache.Cache
	authService services.AuthGrpcService
	wordStore   wordstore.Store
}

func New(config util.Config, store db.Store, cache commonCache.Cache, authService services.AuthGrpcService) *Handler {
	wordStore, err := wordstore.NewDefaultStore()
	if err != nil {
		log.Fatal().Err(err).Msg("could not initialize word store")
	}

	return &Handler{
		config:      config,
		store:       store,
		cache:       cache,
		authService: authService,
		wordStore:   wordStore,
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

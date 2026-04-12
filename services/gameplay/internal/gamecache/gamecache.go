package gamecache

import (
	commonCache "github.com/spazzle-io/spazzle-api/libs/common/cache"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/util"
)

type GameCache struct {
	config *util.Config
	cache  commonCache.Cache
}

func New(config *util.Config, cache commonCache.Cache) *GameCache {
	return &GameCache{
		config: config,
		cache:  cache,
	}
}

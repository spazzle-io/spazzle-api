package gamecache

import (
	commonCache "github.com/spazzle-io/spazzle-api/libs/common/cache"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/util"
)

func getTestConfig() util.Config {
	return util.Config{
		ServiceName: "test",
		Environment: "development",
	}
}

func newTestGameCache(cache commonCache.Cache) *GameCache {
	return New(getTestConfig(), cache)
}

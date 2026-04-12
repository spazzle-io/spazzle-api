package gamecache

import (
	commonCache "github.com/spazzle-io/spazzle-api/libs/common/cache"
	commonConfig "github.com/spazzle-io/spazzle-api/libs/common/config"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/util"
)

func getTestConfig() *util.Config {
	return &util.Config{
		AppConfig: commonConfig.AppConfig{
			ServiceName: "test",
			Environment: "development",
		},
	}
}

func newTestGameCache(cache commonCache.Cache) *GameCache {
	return New(getTestConfig(), cache)
}

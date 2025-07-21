package worker

import (
	"os"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog/log"
	commonCache "github.com/spazzle-io/spazzle-api/libs/common/cache"
	commonConfig "github.com/spazzle-io/spazzle-api/libs/common/config"
	"github.com/spazzle-io/spazzle-api/services/users/internal/util"
)

var (
	testTaskDistributor TaskDistributor
	testRedisCache      commonCache.Cache
)

func TestMain(m *testing.M) {
	config, err := commonConfig.LoadConfig[util.Config]("../../", ".development")
	if err != nil {
		log.Fatal().Err(err).Msg("could not load config")
	}

	redisOpt, err := asynq.ParseRedisURI(config.RedisConnURL)
	if err != nil {
		log.Fatal().Err(err).Msg("could not parse redis URL")
	}

	testRedisCache, err = commonCache.NewRedisCache(config.RedisConnURL)
	if err != nil {
		log.Fatal().Err(err).Msg("could not create new redis cache")
	}

	testTaskDistributor = NewRedisTaskDistributor(redisOpt)

	go runTestTaskProcessor(config, redisOpt, testRedisCache)

	os.Exit(m.Run())
}

func runTestTaskProcessor(config util.Config, redisOpt asynq.RedisConnOpt, redisCache commonCache.Cache) {
	taskProcessor := NewRedisTaskProcessor(redisOpt, config, redisCache)

	err := taskProcessor.Start()
	if err != nil {
		log.Fatal().Err(err).Msg("could not start test task processor")
	}

	log.Info().Msg("started test task processor")
}

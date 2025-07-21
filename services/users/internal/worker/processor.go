package worker

import (
	"context"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
	commonCache "github.com/spazzle-io/spazzle-api/libs/common/cache"
	"github.com/spazzle-io/spazzle-api/services/users/internal/util"
)

const (
	QueueDefault  = "default"
	QueueCritical = "critical"
)

type TaskProcessor interface {
	Start() error
	Shutdown()
}

type RedisTaskProcessor struct {
	server *asynq.Server
	config util.Config
	cache  commonCache.Cache
}

func NewRedisTaskProcessor(redisOpt asynq.RedisConnOpt, config util.Config, cache commonCache.Cache) TaskProcessor {
	logger := NewLogger()
	redis.SetLogger(logger)

	server := asynq.NewServer(redisOpt, asynq.Config{
		Queues: map[string]int{
			QueueDefault:  3,
			QueueCritical: 7,
		},
		ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
			log.Error().Err(err).
				Str("type", task.Type()).
				Bytes("payload", task.Payload()).
				Msg("process task failed")
		}),
		Logger: logger,
	})

	return &RedisTaskProcessor{
		server: server,
		config: config,
		cache:  cache,
	}
}

func (processor *RedisTaskProcessor) Start() error {
	mux := asynq.NewServeMux()

	return processor.server.Start(mux)
}

func (processor *RedisTaskProcessor) Shutdown() {
	processor.server.Shutdown()
}

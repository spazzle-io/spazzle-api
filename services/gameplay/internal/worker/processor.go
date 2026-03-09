package worker

import (
	"context"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
	commonStorage "github.com/spazzle-io/spazzle-api/libs/common/storage"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/eventbus"
)

type TaskProcessor interface {
	Start() error
	ProcessTaskArchiveGame(ctx context.Context, task *asynq.Task) error
}

type RedisTaskProcessor struct {
	server      *asynq.Server
	bus         eventbus.EventBus
	objectStore commonStorage.ObjectStore
}

func NewRedisTaskProcessor(
	redisOpt asynq.RedisConnOpt,
	bus eventbus.EventBus,
	objectStore commonStorage.ObjectStore,
) TaskProcessor {
	logger := NewLogger()
	redis.SetLogger(logger)

	server := asynq.NewServer(
		redisOpt,
		asynq.Config{
			ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
				log.Error().Err(err).
					Str("type", task.Type()).
					Bytes("payload", task.Payload()).
					Msg("could not process task")
			}),
			Logger: logger,
		},
	)

	return &RedisTaskProcessor{
		server:      server,
		bus:         bus,
		objectStore: objectStore,
	}
}

func (processor *RedisTaskProcessor) Start() error {
	mux := asynq.NewServeMux()

	mux.HandleFunc(TaskArchiveGame, processor.ProcessTaskArchiveGame)

	return processor.server.Start(mux)
}

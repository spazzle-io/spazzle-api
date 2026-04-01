package worker

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	db "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/sqlc"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
	commonStorage "github.com/spazzle-io/spazzle-api/libs/common/storage"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/eventbus"
)

type TaskProcessor interface {
	Start() error
	Stop()
	ProcessTaskArchiveGame(ctx context.Context, task *asynq.Task) error
	ProcessTaskRecomputeTrending(ctx context.Context, task *asynq.Task) error
}

type scheduledTask struct {
	interval         string
	retentionPercent float64
	taskType         string
	payload          []byte
}

var scheduledTasks = []scheduledTask{
	{
		interval:         "@every 15m",
		retentionPercent: 0.8,
		taskType:         TaskRecomputeServerTrendingScores,
	},
}

type RedisTaskProcessor struct {
	server      *asynq.Server
	scheduler   *asynq.Scheduler
	bus         eventbus.EventBus
	store       db.Store
	objectStore commonStorage.ObjectStore
}

func NewRedisTaskProcessor(
	redisOpt asynq.RedisConnOpt,
	bus eventbus.EventBus,
	store db.Store,
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

	scheduler := asynq.NewScheduler(redisOpt, &asynq.SchedulerOpts{
		PostEnqueueFunc: schedulerEnqueueErrorHandler(),
		Logger:          logger,
	})

	return &RedisTaskProcessor{
		server:      server,
		scheduler:   scheduler,
		bus:         bus,
		store:       store,
		objectStore: objectStore,
	}
}

func (processor *RedisTaskProcessor) Start() error {
	mux := asynq.NewServeMux()

	mux.HandleFunc(TaskArchiveGame, processor.ProcessTaskArchiveGame)
	mux.HandleFunc(TaskRecomputeServerTrendingScores, processor.ProcessTaskRecomputeTrending)

	err := processor.registerScheduledTasks()
	if err != nil {
		return err
	}

	go func() {
		if err := processor.scheduler.Run(); err != nil {
			log.Error().Err(err).Msg("failed to start task scheduler")
		}
	}()

	return processor.server.Start(mux)
}

func (processor *RedisTaskProcessor) Stop() {
	processor.scheduler.Shutdown()
	processor.server.Shutdown()
	log.Info().Msg("stopped task processor")
}

func (processor *RedisTaskProcessor) registerScheduledTasks() error {
	for _, st := range scheduledTasks {
		duration, err := parseIntervalDuration(st.interval)
		if err != nil {
			return fmt.Errorf("failed to parse interval for %s: %w", st.taskType, err)
		}

		retention := time.Duration(float64(duration) * st.retentionPercent)

		_, err = processor.scheduler.Register(
			st.interval,
			asynq.NewTask(st.taskType, st.payload),
			asynq.TaskID(st.taskType),
			asynq.MaxRetry(5),
			asynq.Timeout(duration),
			asynq.Retention(retention),
		)
		if err != nil {
			return fmt.Errorf("failed to register scheduled task %s: %w", st.taskType, err)
		}
	}

	return nil
}

func schedulerEnqueueErrorHandler() func(info *asynq.TaskInfo, err error) {
	return func(info *asynq.TaskInfo, err error) {
		if err == nil || errors.Is(err, asynq.ErrTaskIDConflict) || errors.Is(err, asynq.ErrDuplicateTask) {
			return
		}

		e := log.Error().Err(err)

		if info != nil {
			e = e.
				Str("type", info.Type).
				Bytes("payload", info.Payload)
		}

		e.Msg("failed to enqueue scheduled task")
	}
}

func parseIntervalDuration(interval string) (time.Duration, error) {
	if !strings.HasPrefix(interval, "@every ") {
		return 0, fmt.Errorf("unsupported interval format: %s, only @every syntax is supported", interval)
	}

	s := strings.TrimPrefix(interval, "@every ")
	return time.ParseDuration(s)
}

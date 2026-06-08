package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/treasury"

	db "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/sqlc"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
	commonStorage "github.com/spazzle-io/spazzle-api/libs/common/storage"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/eventbus"
)

type TaskProcessor interface {
	StartWorker() error
	StartScheduler() error
	Stop()
}

type scheduledTask struct {
	interval string
	taskType string
	payload  any
	timeout  time.Duration
	maxRetry int
}

var scheduledTasks = []scheduledTask{
	{
		interval: "@every 15m",
		taskType: TaskRecomputeServerTrendingScores,
		timeout:  10 * time.Minute,
		maxRetry: 5,
	},
}

type RedisTaskProcessor struct {
	server         *asynq.Server
	scheduler      *asynq.Scheduler
	bus            eventbus.EventBus
	store          db.Store
	objectStore    commonStorage.ObjectStore
	treasuryClient treasury.Client
}

func NewRedisTaskProcessor(
	redisOpt asynq.RedisConnOpt,
	bus eventbus.EventBus,
	store db.Store,
	objectStore commonStorage.ObjectStore,
	treasuryClient treasury.Client,
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
		server:         server,
		scheduler:      scheduler,
		bus:            bus,
		store:          store,
		objectStore:    objectStore,
		treasuryClient: treasuryClient,
	}
}

func (p *RedisTaskProcessor) StartWorker() error {
	mux := asynq.NewServeMux()

	mux.HandleFunc(TaskArchiveGame, p.processTaskArchiveGame)
	mux.HandleFunc(TaskDeployTreasury, p.processTaskDeployTreasury)
	mux.HandleFunc(TaskRecomputeServerTrendingScores, p.processTaskRecomputeTrending)

	if err := p.server.Run(mux); err != nil {
		if errors.Is(err, asynq.ErrServerClosed) {
			return nil
		}

		return err
	}

	return nil
}

func (p *RedisTaskProcessor) StartScheduler() error {
	if err := p.registerScheduledTasks(); err != nil {
		return err
	}

	if err := p.scheduler.Run(); err != nil {
		return fmt.Errorf("task scheduler runtime error: %w", err)
	}

	return nil
}

func (p *RedisTaskProcessor) Stop() {
	if p == nil {
		return
	}

	p.scheduler.Shutdown()
	p.server.Shutdown()

	log.Info().Msg("stopped task processor")
}

func (p *RedisTaskProcessor) registerScheduledTasks() error {
	for _, st := range scheduledTasks {
		intervalDuration, err := parseIntervalDuration(st.interval)
		if err != nil {
			return fmt.Errorf("failed to parse interval duration for %s: %w", st.taskType, err)
		}

		if st.timeout >= intervalDuration {
			return fmt.Errorf(
				"invalid scheduled task %s: timeout (%v) must be strictly less than its interval frequency (%v)",
				st.taskType, st.timeout, intervalDuration,
			)
		}

		var payloadBytes []byte
		if st.payload != nil {
			payloadBytes, err = json.Marshal(st.payload)
			if err != nil {
				return fmt.Errorf("failed to marshal payload config for %s: %w", st.taskType, err)
			}
		}

		_, err = p.scheduler.Register(
			st.interval,
			asynq.NewTask(st.taskType, payloadBytes),
			asynq.TaskID(st.taskType),
			asynq.MaxRetry(st.maxRetry),
			asynq.Timeout(st.timeout),
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

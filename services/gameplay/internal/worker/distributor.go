package worker

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"
)

type TaskDistributor interface {
	DistributeTaskArchiveGame(ctx context.Context, payload *PayloadArchiveGame, opts ...asynq.Option) error
	DistributeTaskDeployTreasury(ctx context.Context, payload *PayloadDeployTreasury, opts ...asynq.Option) error
	Close() error
}

type RedisTaskDistributor struct {
	client *asynq.Client
}

func NewRedisTaskDistributor(redisOpt asynq.RedisConnOpt) TaskDistributor {
	client := asynq.NewClient(redisOpt)
	return &RedisTaskDistributor{
		client: client,
	}
}

func (distributor *RedisTaskDistributor) Close() error {
	if err := distributor.client.Close(); err != nil {
		return fmt.Errorf("could not close task distributor: %w", err)
	}

	return nil
}

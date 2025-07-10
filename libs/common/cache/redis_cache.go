package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisCache struct {
	client *redis.Client
}

func NewRedisCache(connURL string) (Cache, error) {
	opts, err := redis.ParseURL(connURL)
	if err != nil {
		return nil, fmt.Errorf("could not parse redis connection url: %w", err)
	}

	rc := &RedisCache{
		client: redis.NewClient(opts),
	}

	return rc, nil
}

func (rc *RedisCache) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	serializedValue, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("could not serialize value: %w", err)
	}
	return rc.client.Set(ctx, key, serializedValue, expiration).Err()
}

func (rc *RedisCache) Get(ctx context.Context, key string) (interface{}, error) {
	res, err := rc.client.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("could not get value: %w", err)
	}

	var deserializedValue interface{}
	err = json.Unmarshal([]byte(res), &deserializedValue)
	if err != nil {
		return nil, fmt.Errorf("could not deserialize value: %w", err)
	}

	return deserializedValue, nil
}

func (rc *RedisCache) Del(ctx context.Context, key string) error {
	_, err := rc.client.Del(ctx, key).Result()
	if err != nil {
		return fmt.Errorf("could not delete key in redis cache: %w", err)
	}

	return nil
}

func (rc *RedisCache) Close() error {
	return rc.client.Close()
}

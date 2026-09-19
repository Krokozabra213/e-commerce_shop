package stateStore

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisStateStore struct {
	client redis.UniversalClient
	prefix string
}

func NewRedisStateStore(client redis.UniversalClient) *RedisStateStore {
	return &RedisStateStore{
		client: client,
		prefix: "oauth:state:",
	}
}

func (s *RedisStateStore) Set(ctx context.Context, state string, ttl time.Duration) error {
	key := s.prefix + state
	return s.client.Set(ctx, key, "1", ttl).Err()
}

func (s *RedisStateStore) Validate(ctx context.Context, state string) (bool, error) {
	key := s.prefix + state
	exists, err := s.client.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("redis exists: %w", err)
	}
	return exists > 0, nil
}

func (s *RedisStateStore) Delete(ctx context.Context, state string) error {
	key := s.prefix + state
	return s.client.Del(ctx, key).Err()
}

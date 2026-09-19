package ratelimit

import (
	"context"
	_ "embed"
	"fmt"
	"log/slog"
	"time"

	infraConfig "github.com/Krokozabra213/e-commerce_shop/infra/config"
	"github.com/redis/go-redis/v9"
)

//go:embed leakybucket.lua
var luaScript string

type Limiter struct {
	rdb       redis.UniversalClient
	keyPrefix string
	script    *redis.Script
	log       *slog.Logger
}

func New(rdb redis.UniversalClient, keyPrefix string, log *slog.Logger) (*Limiter, error) {
	return &Limiter{
		rdb:       rdb,
		keyPrefix: keyPrefix,
		script:    redis.NewScript(luaScript),
		log:       log,
	}, nil
}

type Result struct {
	Allowed    bool
	Remaining  int
	RetryAfter time.Duration
	Limit      int
}

func (l *Limiter) Allow(ctx context.Context, key string, rule infraConfig.RateLimitRule) (*Result, error) {
	redisKey := fmt.Sprintf("%s:%s", l.keyPrefix, key)
	now := time.Now().UnixNano()
	ttlSec := int(rule.Window.Seconds())

	res, err := l.script.Run(ctx, l.rdb,
		[]string{redisKey},
		rule.Capacity,
		rule.Rate,
		now,
		ttlSec,
	).Slice()

	if err != nil {
		return nil, fmt.Errorf("ratelimit redis: %w", err)
	}

	if len(res) < 3 {
		return nil, fmt.Errorf("ratelimit redis: invalid response length: got %d, want 3", len(res))
	}

	allowedVal, ok1 := res[0].(int64)
	remainingVal, ok2 := res[1].(int64)
	retryAfterMs, ok3 := res[2].(int64)

	if !ok1 || !ok2 || !ok3 {
		return nil, fmt.Errorf("ratelimit redis: invalid response types from lua script")
	}

	return &Result{
		Allowed:    allowedVal == 1,
		Remaining:  int(remainingVal),
		RetryAfter: time.Duration(retryAfterMs) * time.Millisecond,
		Limit:      rule.Capacity,
	}, nil
}

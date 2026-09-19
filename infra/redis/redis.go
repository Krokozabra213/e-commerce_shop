package infraredis

import (
	"context"
	"fmt"
	"log/slog"

	infracfg "github.com/Krokozabra213/e-commerce_shop/infra/config"
	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
)

func NewClient(cfg infracfg.RedisConfig, log *slog.Logger) (redis.UniversalClient, error) {

	rdb := redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs:    cfg.Addrs(),
		Password: cfg.Password,
		DB:       cfg.Database,

		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,

		PoolSize:        cfg.PoolSize,
		MinIdleConns:    cfg.MinIdleConns,
		ConnMaxIdleTime: cfg.ConnMaxIdleTime,

		MaxRetries:      cfg.MaxRetries,
		MinRetryBackoff: cfg.MinRetryBackoff,
		MaxRetryBackoff: cfg.MaxRetryBackoff,
	})

	if err := redisotel.InstrumentTracing(rdb); err != nil {
		return nil, fmt.Errorf("redis otel: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.DialTimeout)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		_ = rdb.Close()
		return nil, fmt.Errorf("redis ping: %w", err)
	}

	log.Info("redis connected", "addr", cfg.Addr)

	return rdb, nil
}

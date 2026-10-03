package infracfg

import (
	"fmt"
	"time"
)

type RedisConfig struct {
	Addr     string `yaml:"addr"     env:"ADDR"     env-default:"localhost:6379" env-required:"true"`
	Password string `yaml:"password" env:"PASSWORD"`
	Database int    `yaml:"database" env:"DB"       env-default:"0"`

	DialTimeout  time.Duration `yaml:"dialTimeout"  env-default:"5s"`
	ReadTimeout  time.Duration `yaml:"readTimeout"  env-default:"3s"`
	WriteTimeout time.Duration `yaml:"writeTimeout" env-default:"3s"`

	PoolSize        int           `yaml:"poolSize"        env-default:"20"`
	MinIdleConns    int           `yaml:"minIdleConns"    env-default:"5"`
	ConnMaxIdleTime time.Duration `yaml:"connMaxIdleTime" env-default:"5m"`

	MaxRetries      int           `yaml:"maxRetries"      env-default:"3"`
	MinRetryBackoff time.Duration `yaml:"minRetryBackoff" env-default:"8ms"`
	MaxRetryBackoff time.Duration `yaml:"maxRetryBackoff" env-default:"512ms"`
}

func (c RedisConfig) Addrs() []string {
	return []string{c.Addr}
}

func (c RedisConfig) Validate() error {
	if c.Addr == "" {
		return fmt.Errorf("redis addr is required")
	}
	if c.PoolSize <= 0 {
		return fmt.Errorf("redis poolSize must be > 0, got %d", c.PoolSize)
	}
	if c.MinIdleConns > c.PoolSize {
		return fmt.Errorf("redis minIdleConns (%d) > poolSize (%d)", c.MinIdleConns, c.PoolSize)
	}
	return nil
}

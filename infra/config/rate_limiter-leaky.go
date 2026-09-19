package infracfg

import (
	"fmt"
	"time"
)

type RateLimitConfig struct {
	Enabled bool `yaml:"enabled" env-default:"true"`

	Authenticated RateLimitRule `yaml:"authenticated"`

	Anonymous RateLimitRule `yaml:"anonymous"`

	FailOpen bool `yaml:"failOpen" env-default:"true"`

	KeyPrefix string `yaml:"keyPrefix" env-default:"ratelimit"`
}

type RateLimitRule struct {
	Capacity int           `yaml:"capacity"`
	Rate     float64       `yaml:"rate"`
	Window   time.Duration `yaml:"window"`
}

func (c RateLimitConfig) Validate() error {
	if !c.Enabled {
		return nil
	}
	if err := c.Authenticated.validate("authenticated"); err != nil {
		return err
	}
	if err := c.Anonymous.validate("anonymous"); err != nil {
		return err
	}
	return nil
}

func (r RateLimitRule) validate(name string) error {
	if r.Capacity <= 0 {
		return fmt.Errorf("ratelimit %s: capacity must be > 0, got %d", name, r.Capacity)
	}
	if r.Rate <= 0 {
		return fmt.Errorf("ratelimit %s: rate must be > 0, got %f", name, r.Rate)
	}
	if r.Window <= 0 {
		return fmt.Errorf("ratelimit %s: window must be > 0", name)
	}
	return nil
}

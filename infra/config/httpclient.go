package infracfg

import "time"

type HTTPClientConfig struct {
	Addr    string        `yaml:"addr" env-required:"true"`
	Timeout time.Duration `yaml:"timeout"  env-default:"5s"`
	Retries int           `yaml:"retries"  env-default:"2"`
}

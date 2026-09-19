package infracfg

import "time"

type OutboxConfig struct {
	BatchSize    int           `yaml:"batchSize"    env-default:"100"`
	PollInterval time.Duration `yaml:"pollInterval" env-default:"1s"`
	MaxAttempts  int           `yaml:"maxAttempts"  env-default:"10"`
	ErrorBackoff time.Duration `yaml:"errorBackoff" env-default:"2s"`
	DBTimeout    time.Duration `yaml:"dbTimeout"    env-default:"5s"`
	SendTimeout  time.Duration `yaml:"sendTimeout"  env-default:"5s"`
	Lease        time.Duration `yaml:"lease"        env-default:"1m"`
}

package config

import "time"

type EmailVerificationConfig struct {
	EmailVerificationTTL    time.Duration `yaml:"emailVerificationTTL"      env-default:"15m"`
	VerificationTokenLength int           `yaml:"verificationTokenLength"  env-default:"32"`
}

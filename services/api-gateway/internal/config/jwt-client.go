package config

import (
	"fmt"
	"time"

	infracfg "github.com/Krokozabra213/e-commerce_shop/infra/config"
)

type JWTClientConfig struct {
	infracfg.JWTConfig `yaml:",inline"`

	PublicKeyPath string `yaml:"publicKeyPath" env:"JWT_PUBLIC_KEY_PATH"`
	JWKSEndpoint  string `yaml:"jwksEndpoint"  env:"JWT_JWKS_ENDPOINT"`

	Leeway time.Duration `yaml:"leeway" env-default:"30s"`

	JWKSCacheTTL    time.Duration `yaml:"jwksCacheTTL"    env-default:"1h"`
	JWKSRefreshRate time.Duration `yaml:"jwksRefreshRate" env-default:"5m"`
}

func (c JWTClientConfig) Validate() error {
	if err := c.JWTConfig.Validate(); err != nil {
		return err
	}
	if c.PublicKeyPath == "" && c.JWKSEndpoint == "" {
		return fmt.Errorf("jwt client: either publicKeyPath or jwksEndpoint is required")
	}
	if c.PublicKeyPath != "" && c.JWKSEndpoint != "" {
		return fmt.Errorf("jwt client: use either publicKeyPath or jwksEndpoint, not both")
	}
	if c.Leeway < 0 {
		return fmt.Errorf("jwt client: leeway must be >= 0")
	}
	if c.Leeway > 2*time.Minute {
		return fmt.Errorf("jwt client: leeway %s is too large (max 2m)", c.Leeway)
	}
	return nil
}

package config

import (
	"fmt"
	"time"

	infracfg "github.com/Krokozabra213/e-commerce_shop/infra/config"
)

type AuthJWTConfig struct {
	infracfg.JWTConfig `yaml:",inline"`

	PrivateKeyPath string `yaml:"privateKeyPath" env:"PRIVATE_KEY_PATH" env-required:"true"`
	KeyID          string `yaml:"keyID"          env:"KEY_ID"           env-required:"true"`

	AccessTTL  time.Duration `yaml:"accessTTL"  env-default:"15m"`
	RefreshTTL time.Duration `yaml:"refreshTTL" env-default:"720h"`

	Rotation RotationConfig `yaml:"rotation"`
}

type RotationConfig struct {
	Enabled        bool          `yaml:"enabled"        env-default:"false"`
	CheckInterval  time.Duration `yaml:"checkInterval"  env-default:"1h"`
	PrivateKeyPath string        `yaml:"privateKeyPath" env:"JWT_ROTATION_PRIVATE_KEY_PATH"`
	NewKeyID       string        `yaml:"newKeyID"       env:"JWT_ROTATION_KEY_ID"`
}

func (c AuthJWTConfig) Validate() error {
	if err := c.JWTConfig.Validate(); err != nil {
		return err
	}
	if c.PrivateKeyPath == "" {
		return fmt.Errorf("jwt: privateKeyPath is required")
	}
	if c.KeyID == "" {
		return fmt.Errorf("jwt: keyID is required")
	}
	if c.AccessTTL <= 0 {
		return fmt.Errorf("jwt: accessTTL must be > 0")
	}
	if c.RefreshTTL <= 0 {
		return fmt.Errorf("jwt: refreshTTL must be > 0")
	}
	if c.AccessTTL >= c.RefreshTTL {
		return fmt.Errorf("jwt: accessTTL (%s) must be < refreshTTL (%s)", c.AccessTTL, c.RefreshTTL)
	}
	return nil
}

package infracfg

import "fmt"

type JWTConfig struct {
	Algorithm string `yaml:"algorithm" env-default:"RS256"`
	Issuer    string `yaml:"issuer"    env:"ISSUER" env-required:"true"`
}

func (c JWTConfig) Validate() error {
	switch c.Algorithm {
	case "RS256", "RS384", "RS512", "PS256", "PS384", "PS512":
	default:
		return fmt.Errorf("jwt: unsupported algorithm %q", c.Algorithm)
	}
	if c.Issuer == "" {
		return fmt.Errorf("jwt: issuer is required")
	}
	return nil
}

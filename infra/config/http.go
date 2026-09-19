package infracfg

import "time"

type HTTPConfig struct {
	Host           string        `yaml:"host" env:"HOST" env-default:"0.0.0.0"`
	Port           string        `yaml:"port" env:"PORT" env-default:"8080"`
	ReadTimeout    time.Duration `yaml:"readTimeout" env:"READ_TIMEOUT" env-default:"10s"`
	WriteTimeout   time.Duration `yaml:"writeTimeout" env:"WRITE_TIMEOUT" env-default:"10s"`
	MaxHeaderBytes int           `yaml:"maxHeaderBytes" env:"MAX_HEADER_BYTES" env-default:"1"`
	BodyLimitBytes int           `yaml:"bodyLimitBytes" env:"BODY_LIMIT_BYTES" env-default:"4"`
	AllowedOrigins []string      `yaml:"allowedOrigins" env:"ALLOWED_ORIGINS" env-separator:","`
}

func (c HTTPConfig) HTTPAddress() string {
	return c.Host + ":" + c.Port
}

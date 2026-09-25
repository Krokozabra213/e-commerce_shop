package infracfg

import "time"

type GRPCConfig struct {
	Host           string        `yaml:"host" env:"HOST" env-default:"0.0.0.0"`
	Port           string        `yaml:"port" env:"PORT"`
	ReadTimeout    time.Duration `yaml:"readTimeout" env:"READ_TIMEOUT" env-default:"10s"`
	WriteTimeout   time.Duration `yaml:"writeTimeout" env:"WRITE_TIMEOUT" env-default:"10s"`
	MaxHeaderBytes int           `yaml:"maxHeaderBytes" env:"MAX_HEADER_BYTES" env-default:"1"`
}

func (c GRPCConfig) GRPCAddress() string {
	return c.Host + ":" + c.Port
}

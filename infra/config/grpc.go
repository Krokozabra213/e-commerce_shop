package infracfg

import "time"

type GRPCConfig struct {
	Host           string        `yaml:"host" env:"GRPC_HOST" env-default:"0.0.0.0"`
	Port           string        `yaml:"port" env:"GRPC_PORT" env-default:"44050"`
	ReadTimeout    time.Duration `yaml:"readTimeout" env:"GRPC_READ_TIMEOUT" env-default:"10s"`
	WriteTimeout   time.Duration `yaml:"writeTimeout" env:"GRPC_WRITE_TIMEOUT" env-default:"10s"`
	MaxHeaderBytes int           `yaml:"maxHeaderBytes" env:"GRPC_MAX_HEADER_BYTES" env-default:"1"`
}

func (c GRPCConfig) GRPCAddress() string {
	return c.Host + ":" + c.Port
}

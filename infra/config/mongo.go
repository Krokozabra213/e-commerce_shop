package infracfg

import "time"

type MongoDBConfig struct {
	URI      string `yaml:"-" env:"URI" env-required:"true"`
	Database string `yaml:"database" env:"DATABASE" env-required:"true"`

	MaxPoolSize     uint64        `yaml:"maxPoolSize" env-default:"100"`
	MinPoolSize     uint64        `yaml:"minPoolSize" env-default:"10"`
	MaxConnIdleTime time.Duration `yaml:"maxConnIdleTime" env-default:"30m"`
	ConnectTimeout  time.Duration `yaml:"connectTimeout" env-default:"10s"`
	SocketTimeout   time.Duration `yaml:"socketTimeout" env-default:"30s"`
	ServerTimeout   time.Duration `yaml:"serverTimeout" env-default:"30s"`

	RetryWrites bool `yaml:"retry_writes" env-default:"true"`

	Compressors []string `yaml:"compressors" env-default:"snappy,zstd"`

	AppName string `yaml:"appName" env:"APP_NAME" env-required:"true"`
}

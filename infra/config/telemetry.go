package infracfg

type TelemetryConfig struct {
	Enabled          bool    `yaml:"enabled" env:"ENABLED"`
	Logs             bool    `yaml:"logs" env:"LOGS"`
	Metrics          bool    `yaml:"metrics"  env:"METRICS"`
	Traces           bool    `yaml:"traces" env:"TRACES"`
	EndPoint         string  `yaml:"endpoint" env:"ENDPOINT"`
	Insecure         bool    `yaml:"insecure" env:"INSECURE"`
	ServiceName      string  `yaml:"service_name" env:"SERVICE_NAME"`
	ServiceVersion   string  `yaml:"service_version" env:"SERVICE_VERSION"`
	Environment      string  `yaml:"environment" env:"ENVIRONMENT"`
	TracesSampleRate float64 `yaml:"tracesSampleRate" env:"TRACES_SAMPLE_RATE"`
	LogToStdout      bool    `yaml:"logToStdout" env:"LOG_TO_STDOUT"`
	SampleLogs       bool    `yaml:"sampleLogs"  env:"SAMPLE_LOGS"`
}

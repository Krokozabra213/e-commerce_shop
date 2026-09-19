package infracfg

import "fmt"

type TracesConfig struct {
	Enabled    bool    `yaml:"enabled"    env-default:"true"`
	SampleRate float64 `yaml:"sampleRate" env-default:"1.0"`
}

type MetricsConfig struct {
	Enabled bool `yaml:"enabled" env-default:"true"`
}

type TelemetryConfig struct {
	ServiceName string `yaml:"serviceName" env:"SERVICE_NAME" env-required:"true"`
	Endpoint    string `yaml:"endpoint"    env:"OTEL_ENDPOINT"  env-required:"true"`
	Insecure    bool   `yaml:"insecure"    env-default:"false"`

	Traces   TracesConfig  `yaml:"traces"`
	Metrics  MetricsConfig `yaml:"metrics"`
	OTelLogs bool          `yaml:"otelLogs" env-default:"false"` // OTel Logs signal (не stdout!)
}

func (c TelemetryConfig) Validate() error {
	if c.ServiceName == "" {
		return fmt.Errorf("serviceName is required")
	}
	if c.Endpoint == "" {
		return fmt.Errorf("endpoint is required")
	}
	if c.Traces.SampleRate < 0 || c.Traces.SampleRate > 1 {
		return fmt.Errorf("tracesSampleRate must be between 0.0 and 1.0, got %f", c.Traces.SampleRate)
	}
	return nil
}

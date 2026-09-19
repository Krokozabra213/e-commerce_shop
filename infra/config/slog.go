package infracfg

import "fmt"

type SlogConfig struct {
	Level     string `yaml:"level"      env-default:"info"`
	Format    string `yaml:"format"     env-default:"json"`
	AddSource bool   `yaml:"add_source" env-default:"false"`
}

func (c SlogConfig) Validate() error {
	switch c.Level {
	case "debug", "info", "warn", "warning", "error":
		// ok
	default:
		return fmt.Errorf("invalid log level %q", c.Level)
	}

	switch c.Format {
	case "json", "text":
		// ok
	default:
		return fmt.Errorf("invalid log format %q", c.Format)
	}

	return nil
}

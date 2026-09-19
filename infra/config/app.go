package infracfg

type AppConfig struct {
	Name    string `yaml:"name" env-required:"true"`
	Version string `yaml:"version" env-default:"v1.0.0"`
	ENV     string `env:"ENV" env-default:"prod"`
	Secret  string `env:"SECRET" env-required:"true"`
}

package infracfg

type GRPCClientConfig struct {
	Addr string `yaml:"addr" env-required:"true"`
}

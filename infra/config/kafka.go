package infracfg

import "time"

type KafkaConsumerConfig struct {
	Brokers []string `yaml:"brokers" env:"BROKERS" env-required:"true" env-separator:","`
	GroupID string   `yaml:"groupID" env:"GROUP_ID" env-required:"true"`
	Topic   string   `yaml:"topic"   env:"TOPIC"    env-required:"true"`

	DLQTopic string `yaml:"dlqTopic" env:"DLQ_TOPIC"`

	SessionTimeout      time.Duration `yaml:"sessionTimeout"    env-default:"30s"`
	HeartbeatInterval   time.Duration `yaml:"heartbeatInterval" env-default:"10s"`
	MaxPollInterval     time.Duration `yaml:"maxPollInterval"   env-default:"5m"`
	StartOffsetEarliest bool          `yaml:"StartOffsetEarliest"       env-default:"true"`
}

type KafkaProducerConfig struct {
	Brokers      []string      `yaml:"brokers"        env:"BROKERS" env-separator:","`
	ClientID     string        `yaml:"clientID"       env:"CLIENT_ID"`
	DialTimeout  time.Duration `yaml:"dialTimeout"    env-default:"5s"`
	WriteTimeout time.Duration `yaml:"writeTimeout"   env-default:"10s"`
	MaxRetries   int           `yaml:"maxRetries"     env-default:"5"`
	RetryBackoff time.Duration `yaml:"retryBackoff"   env-default:"300ms"`
}

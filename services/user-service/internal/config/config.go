package config

import (
	"fmt"
	"log"
	"os"

	infracfg "github.com/Krokozabra213/e-commerce_shop/infra/config"
	"github.com/ilyakaznacheev/cleanenv"
	"github.com/joho/godotenv"
)

type Config struct {
	App            infracfg.AppConfig            `yaml:"app" env-prefix:"APP_"`
	Postgres       infracfg.PostgresConfig       `yaml:"postgres" env-prefix:"POSTGRES_"`
	HTTP           infracfg.HTTPConfig           `yaml:"http" env-prefix:"HTTP_"`
	Logger         infracfg.SlogConfig           `yaml:"slog" env-prefix:"SLOG_"`
	KafkaConsumer  infracfg.KafkaConsumerConfig  `yaml:"kafkaConsumer" env-prefix:"KAFKA_CONSUMER"`
	SchemaRegistry infracfg.SchemaRegistryConfig `yaml:"schema_registry" env-prefix:"SCHEMA_REGISTRY_"`
}

func Init() (*Config, error) {
	if err := godotenv.Load(); err != nil {
		log.Print(err)
	}

	env := os.Getenv("ENV")
	if env == "" {
		env = "dev"
	}

	configFile := fmt.Sprintf("configs/%s.yaml", env)

	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("config file not found: %s (ENV=%s)", configFile, env)
	}

	var cfg Config

	if err := cleanenv.ReadConfig(configFile, &cfg); err != nil {
		return nil, err
	}

	err := cfg.Validate()
	if err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *Config) Validate() error {
	err := c.Logger.Validate()
	if err != nil {
		return err
	}
	return nil
}

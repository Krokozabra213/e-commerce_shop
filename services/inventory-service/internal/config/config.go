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
	App                         infracfg.AppConfig           `yaml:"app" env-prefix:"APP_"`
	Postgres                    infracfg.PostgresConfig      `yaml:"postgres" env-prefix:"POSTGRES_"`
	HTTP                        infracfg.HTTPConfig          `yaml:"http" env-prefix:"HTTP_"`
	GRPC                        infracfg.GRPCConfig          `yaml:"grpc" env-prefix:"GRPC_"`
	Logger                      infracfg.SlogConfig          `yaml:"slog" env-prefix:"SLOG_"`
	OrderCreatedConsumer        infracfg.KafkaConsumerConfig `yaml:"orderCreatedConsumer" env-prefix:"ORDER_CREATED_CONSUMER"`
	OrderCancelledConsumer      infracfg.KafkaConsumerConfig `yaml:"orderCancelledConsumer" env-prefix:"ORDER_CANCELLED_CONSUMER"`
	InventoryReservedOutbox     infracfg.OutboxConfig        `yaml:"inventory_reserved_outbox" env-prefix:"INVENTORY_RESERVED_OUTBOX_"`
	InventoryReservFailedOutbox infracfg.OutboxConfig        `yaml:"inventory_reserv_failed_outbox" env-prefix:"INVENTORY_RESERV_FAILED_OUTBOX_"`
	KafkaProducer               infracfg.KafkaProducerConfig `yaml:"kafka_producer" env-prefix:"KAFKA_"`
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

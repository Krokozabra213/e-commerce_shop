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
	App                 infracfg.AppConfig        `yaml:"app" env-prefix:"APP_"`
	Redis               infracfg.RedisConfig      `yaml:"redis" env-prefix:"REDIS_"`
	HTTP                infracfg.HTTPConfig       `yaml:"http" env-prefix:"HTTP_"`
	Logger              infracfg.SlogConfig       `yaml:"slog" env-prefix:"SLOG_"`
	HTTPAuthClient      infracfg.HTTPClientConfig `yaml:"http_auth_client" env-prefix:"HTTP_AUTH_CLIENT"`
	HTTPInventoryClient infracfg.HTTPClientConfig `yaml:"http_inventory_client" env-prefix:"HTTP_INVENTION_CLIENT"`
	HTTPOrderClient     infracfg.HTTPClientConfig `yaml:"http_order_client" env-prefix:"HTTP_ORDER_CLIENT"`
	HTTPProductClient   infracfg.HTTPClientConfig `yaml:"http_product_client" env-prefix:"HTTP_PROD_CLIENT"`
	HTTPUserClient      infracfg.HTTPClientConfig `yaml:"http_user_client" env-prefix:"HTTP_USER_CLIENT"`
	GRPCInventoryClient infracfg.GRPCClientConfig `yaml:"grpc_inventory_client" env-prefix:"GRPC_INVENTION_CLIENT"`
	JWTClient           JWTClientConfig           `yaml:"jwt" env-prefix:"JWT_"`
	RateLimiter         infracfg.RateLimitConfig  `yaml:"rate_limiter" env-prefix:"RATE_LIMIT"`
	Telemetry           infracfg.TelemetryConfig  `yaml:"telemetry" env-prefix:"TELEMETRY_"`
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
	err = c.Redis.Validate()
	if err != nil {
		return err
	}
	return nil
}

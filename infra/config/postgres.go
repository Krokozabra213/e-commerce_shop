package infracfg

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

type PostgresConfig struct {
	Host     string `yaml:"host"     env:"HOST"     env-required:"true"`
	Port     string `yaml:"port" env:"PORT" env-required:"true"`
	User     string `yaml:"user"     env:"USER"     env-required:"true"`
	Password string `yaml:"password" env:"PASSWORD" env-required:"true"`
	DBName   string `yaml:"dbName"   env:"DBNAME"   env-required:"true"`
	SSLMode  string `yaml:"sslMode"  env:"SSLMODE"  env-default:"disable"`

	MaxConns          int           `yaml:"maxConns"          env-default:"20"`
	MinConns          int           `yaml:"minConns"          env-default:"5"`
	MaxConnLifetime   time.Duration `yaml:"maxConnLifetime"   env-default:"1h"`
	MaxConnIdleTime   time.Duration `yaml:"maxConnIdleTime"   env-default:"30m"`
	HealthCheckPeriod time.Duration `yaml:"healthCheckPeriod" env-default:"1m"`
	ConnectTimeout    time.Duration `yaml:"connectTimeout"    env-default:"10s"`

	IncludeQueryParams bool `yaml:"includeQueryParams" env-default:"false"`
}

func (c PostgresConfig) PGAddress() string {
	return c.Host + ":" + c.Port
}

func (c *PostgresConfig) DSN() string {
	user := url.PathEscape(c.User)
	password := url.PathEscape(c.Password)
	dbName := url.PathEscape(c.DBName)

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s",
		user, password, c.Host, c.Port, dbName)

	var params []string
	if c.SSLMode != "" {
		params = append(params, "sslmode="+c.SSLMode)
	}

	if c.ConnectTimeout > 0 {
		params = append(params, fmt.Sprintf("connect_timeout=%d", int(c.ConnectTimeout.Seconds())))
	}

	if len(params) > 0 {
		dsn += "?" + strings.Join(params, "&")
	}

	return dsn
}

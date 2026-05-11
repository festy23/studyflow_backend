package config

import (
	"fmt"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	DBHost     string `env:"FAQ_DB_HOST" env-default:"localhost"`
	DBPort     int    `env:"FAQ_DB_PORT" env-default:"5432"`
	DBUser     string `env:"FAQ_DB_USER" env-default:"postgres"`
	DBPassword string `env:"FAQ_DB_PASSWORD" env-default:"postgres"`
	DBName     string `env:"FAQ_DB_NAME" env-default:"faqdb"`
	DBSSLMode  string `env:"FAQ_DB_SSLMODE" env-default:"disable"`
	GRPCPort   string `env:"FAQ_GRPC_PORT" env-default:"50051"`
}

func Load() (*Config, error) {
	var cfg Config
	if err := cleanenv.ReadEnv(&cfg); err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	return &cfg, nil
}

func (c *Config) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.DBHost, c.DBPort, c.DBUser, c.DBPassword, c.DBName, c.DBSSLMode,
	)
}

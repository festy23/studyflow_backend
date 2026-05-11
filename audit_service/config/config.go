package config

import (
	"errors"
	"os"
	"strings"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	GRPCPort            int    `env:"GRPC_PORT" env-default:"50051"`
	PostgresURL         string `env:"POSTGRES_URL" env-default:""`
	PostgresMaxConn     int32  `env:"POSTGRES_MAX_CONN" env-default:"5"`
	PostgresMinConn     int32  `env:"POSTGRES_MIN_CONN" env-default:"1"`
	PostgresAutoMigrate bool   `env:"POSTGRES_AUTO_MIGRATE" env-default:"true"`
}

func New() (*Config, error) {
	var cfg Config
	if err := cleanenv.ReadConfig("./config/.env", &cfg); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if err := cleanenv.ReadEnv(&cfg); err != nil {
				return nil, err
			}
			// Fall through to validation below.
		} else {
			return nil, err
		}
	}

	if cfg.PostgresURL == "" || strings.Contains(cfg.PostgresURL, "CHANGEME") {
		return nil, errors.New("POSTGRES_URL must be set and must not contain CHANGEME")
	}

	return &cfg, nil
}

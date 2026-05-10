package config

import (
	"errors"
	"github.com/ilyakaznacheev/cleanenv"
	"os"
)

type Config struct {
	AppEnv             string `env:"APP_ENV" env-default:"production"`
	HTTPPort           int    `env:"HTTP_PORT" env-default:"8080"`
	UserServiceURL     string `env:"USER_CLIENT_URL"`
	FileServiceURL     string `env:"FILE_SERVICE_URL"`
	ScheduleServiceURL string `env:"SCHEDULE_SERVICE_URL"`
	HomeworkServiceURL string `env:"HOMEWORK_SERVICE_URL"`
	PaymentServiceURL  string `env:"PAYMENT_SERVICE_URL"`
	// MinioURL is the internal MinIO endpoint used for backend-to-backend
	// proxying from this gateway (not exposed to clients).
	MinioURL string `env:"MINIO_URL"`
	// PublicFileURLPrefix is the externally reachable URL prefix used when
	// returning file URLs to clients. Falls back to GATEWAY_PUBLIC_URL.
	PublicFileURLPrefix string `env:"PUBLIC_FILE_URL_PREFIX"`
	GatewayPublicURL    string `env:"GATEWAY_PUBLIC_URL"`
	RedisURL            string `env:"REDIS_URL"`
}

func New() (*Config, error) {
	var cfg Config
	if err := cleanenv.ReadConfig("./config/.env", &cfg); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if err := cleanenv.ReadEnv(&cfg); err != nil {
				return nil, err
			}
			return &cfg, nil
		}
		return nil, err
	}
	return &cfg, nil
}

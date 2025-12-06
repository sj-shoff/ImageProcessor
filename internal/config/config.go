package config

import (
	"fmt"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/ilyakaznacheev/cleanenv"
	"github.com/wb-go/wbf/retry"
)

type Config struct {
	DB struct {
		Host            string        `env:"POSTGRES_HOST" validate:"required"`
		Port            int           `env:"POSTGRES_PORT" validate:"required"`
		User            string        `env:"POSTGRES_USER" validate:"required"`
		Pass            string        `env:"POSTGRES_PASSWORD" validate:"required"`
		DBName          string        `env:"POSTGRES_DB" validate:"required"`
		MaxOpenConns    int           `env:"DB_MAX_OPEN_CONNS"`
		MaxIdleConns    int           `env:"DB_MAX_IDLE_CONNS"`
		ConnMaxLifetime time.Duration `env:"DB_CONN_MAX_LIFETIME"`
	}

	Server struct {
		Addr            string        `env:"SERVER_PORT" validate:"required"`
		ReadTimeout     time.Duration `env:"SERVER_READ_TIMEOUT" validate:"required"`
		WriteTimeout    time.Duration `env:"SERVER_WRITE_TIMEOUT" validate:"required"`
		IdleTimeout     time.Duration `env:"SERVER_IDLE_TIMEOUT" validate:"required"`
		ShutdownTimeout time.Duration `env:"SERVER_SHUTDOWN_TIMEOUT" validate:"required"`
	}

	Retries struct {
		Attempts int     `env:"RETRIES_ATTEMPTS" validate:"required"`
		DelayMs  int     `env:"RETRIES_DELAY_MS" validate:"required"`
		Backoff  float64 `env:"RETRIES_BACKOFF" validate:"required"`
	}

	MinIO struct {
		Endpoint  string `env:"MINIO_ENDPOINT" validate:"required"`
		Region    string `env:"MINIO_REGION" validate:"required"`
		AccessKey string `env:"MINIO_ACCESS_KEY" validate:"required"`
		SecretKey string `env:"MINIO_SECRET_KEY" validate:"required"`
		Bucket    string `env:"MINIO_BUCKET"`
		UseSSL    bool   `env:"MINIO_USE_SSL"`
	}

	Kafka struct {
		Brokers         []string `env:"KAFKA_BROKERS" envSeparator:"," validate:"required"`
		ProcessingTopic string   `env:"KAFKA_PROCESSING_TOPIC"`
		ResultsTopic    string   `env:"KAFKA_RESULTS_TOPIC"`
		GroupID         string   `env:"KAFKA_GROUP_ID"`
	}

	Worker struct {
		Concurrency int `env:"WORKER_CONCURRENCY"`
	}
}

func MustLoad() (*Config, error) {
	var cfg Config

	err := cleanenv.ReadEnv(&cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to read environment variables: %w", err)
	}

	validate := validator.New()
	if err := validate.Struct(cfg); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &cfg, nil
}

func (c *Config) DBDSN() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		c.DB.User, c.DB.Pass, c.DB.Host, c.DB.Port, c.DB.DBName)
}

func (c *Config) MinIOEndpoint() string {
	return c.MinIO.Endpoint
}

func (c *Config) DefaultRetryStrategy() retry.Strategy {
	return retry.Strategy{
		Attempts: c.Retries.Attempts,
		Delay:    time.Duration(c.Retries.DelayMs) * time.Millisecond,
		Backoff:  c.Retries.Backoff,
	}
}

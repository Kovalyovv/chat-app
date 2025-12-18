package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	API struct {
		Port string
	}
	Database struct {
		URL string
	}
	GinMode string
}

func NewFromEnv() *Config {
	_ = godotenv.Load() // читает .env, ошибки можно игнорировать

	cfg := &Config{
		API: struct{ Port string }{
			Port: os.Getenv("API_PORT"),
		},
		Database: struct{ URL string }{
			URL: os.Getenv("DB_URL"),
		},
		GinMode: os.Getenv("GIN_MODE"),
	}

	if cfg.API.Port == "" {
		cfg.API.Port = "8080"
	}
	if cfg.GinMode == "" {
		cfg.GinMode = "debug"
	}

	return cfg
}

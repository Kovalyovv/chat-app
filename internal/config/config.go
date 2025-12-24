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
	AuthService struct {
		Addr string
	}
	GinMode string
}

func NewFromEnv() *Config {
	_ = godotenv.Load()

	cfg := &Config{
		API: struct{ Port string }{
			Port: os.Getenv("API_PORT"),
		},
		Database: struct{ URL string }{
			URL: os.Getenv("DB_URL"),
		},
		AuthService: struct{ Addr string }{
			Addr: os.Getenv("AUTH_SERVICE_ADDR"),
		},
		GinMode: os.Getenv("GIN_MODE"),
	}

	if cfg.API.Port == "" {
		cfg.API.Port = "8080"
	}
	if cfg.AuthService.Addr == "" {
		cfg.AuthService.Addr = "localhost:50001"
	}
	if cfg.GinMode == "" {
		cfg.GinMode = "debug"
	}

	return cfg
}

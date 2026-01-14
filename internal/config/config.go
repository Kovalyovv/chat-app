package config

import (
	"os"
	"strconv"

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
	GinMode        string
	InternalAPIKey string
	HistoryLimit   int
}

func NewFromEnv() *Config {
	_ = godotenv.Load()

	historyLimit, _ := strconv.Atoi(os.Getenv("HISTORY_LIMIT"))
	if historyLimit <= 0 {
		historyLimit = 50 // Default value
	}

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
		GinMode:        os.Getenv("GIN_MODE"),
		InternalAPIKey: os.Getenv("INTERNAL_API_KEY"),
		HistoryLimit:   historyLimit,
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

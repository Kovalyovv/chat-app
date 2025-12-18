// cmd/api/main.go
package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/Kovalyovv/chat-app/internal/config"
	httpDelivery "github.com/Kovalyovv/chat-app/internal/delivery/http"
	"github.com/Kovalyovv/chat-app/internal/logger"
	"github.com/Kovalyovv/chat-app/internal/middleware"
	"github.com/Kovalyovv/chat-app/internal/repository/postgres"
	"github.com/Kovalyovv/chat-app/internal/usecase"
)

func main() {
	_ = godotenv.Load() // automatically loads `.env` when running locally

	logr := logger.New()
	cfg := config.NewFromEnv()

	logr.Infof("starting chat service (port=%s) ...", cfg.API.Port)

	store, err := postgres.New(cfg.Database.URL)
	if err != nil {
		logr.Fatalf("failed to connect to db: %v", err)
	}
	defer store.Close()

	roomUC := usecase.NewRoomUseCase(store)
	messageUC := usecase.NewMessageUseCase(store)

	// middleware (for now stub that reads X-User-ID; later will call Auth gRPC)
	authMW := middleware.NewAuthMiddleware() // returns gin middleware

	srv := httpDelivery.NewServer(cfg, roomUC, messageUC, authMW, logr)

	go func() {
		logr.Infof("http server listen :%s", cfg.API.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logr.Fatalf("http listen error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logr.Info("shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	logr.Info("server stopped")
}

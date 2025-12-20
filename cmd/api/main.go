// cmd/api/main.go
package main

import (
	"context"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/Kovalyovv/chat-app/internal/config"
	httpDelivery "github.com/Kovalyovv/chat-app/internal/delivery/http"
	"github.com/Kovalyovv/chat-app/internal/delivery/ws"
	"github.com/Kovalyovv/chat-app/internal/logger"
	"github.com/Kovalyovv/chat-app/internal/middleware"
	"github.com/Kovalyovv/chat-app/internal/repository/postgres"
	"github.com/Kovalyovv/chat-app/internal/usecase"
)

func main() {
	logr := logger.New()
	cfg := config.NewFromEnv()

	store, err := postgres.New(cfg.Database.URL)
	if err != nil {
		logr.Fatalf("db connection failed: %v", err)
	}
	defer store.Close()

	roomUC := usecase.NewRoomUseCase(store.RoomRepo)
	messageUC := usecase.NewMessageUseCase(store.MessageRepo, store.ChatStateRepo)

	hub := ws.NewHub(messageUC)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go hub.Run(ctx)

	authMW := middleware.NewAuthMiddleware()
	srv := httpDelivery.NewServer(cfg, roomUC, messageUC, authMW, logr, hub)

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logr.Fatalf("listen error: %v", err)
		}
	}()

	<-ctx.Done()
	logr.Info("Shutting down gracefully...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}

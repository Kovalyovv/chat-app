package main

import (
	"context"
	"errors"
	"github.com/Kovalyovv/chat-app/internal/config"
	httpDelivery "github.com/Kovalyovv/chat-app/internal/delivery/http"
	"github.com/Kovalyovv/chat-app/internal/delivery/ws"
	"github.com/Kovalyovv/chat-app/internal/middleware"
	"github.com/Kovalyovv/chat-app/internal/repository/postgres"
	"github.com/Kovalyovv/chat-app/internal/usecase"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
	cfg := config.NewFromEnv()

	store, err := postgres.New(cfg.Database.URL)
	if err != nil {
		slog.Error("db connection failed", "error", err)
		os.Exit(1)
	}
	defer store.Close()

	roomUC := usecase.NewRoomUseCase(store.RoomRepo)
	messageUC := usecase.NewMessageUseCase(store.MessageRepo, store.ChatStateRepo)

	hub := ws.NewHub(messageUC, logger.With("component", "hub"))

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go hub.Run(ctx)

	authMW, authConn := middleware.NewAuthMiddleware(cfg.AuthService.Addr, logger)
	defer authConn.Close()

	srv := httpDelivery.NewServer(cfg, roomUC, messageUC, authMW, logger, hub)

	go func() {
		slog.Info("starting HTTP server", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("http server error", "error", err)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down gracefully")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}

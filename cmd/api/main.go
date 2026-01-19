package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Kovalyovv/auth-service/pkg/observability"
	"github.com/Kovalyovv/chat-app/internal/bus"
	"github.com/Kovalyovv/chat-app/internal/config"
	httpDelivery "github.com/Kovalyovv/chat-app/internal/delivery/http"
	"github.com/Kovalyovv/chat-app/internal/delivery/ws"
	"github.com/Kovalyovv/chat-app/internal/domain"
	"github.com/Kovalyovv/chat-app/internal/middleware"
	"github.com/Kovalyovv/chat-app/internal/repository/postgres"
	"github.com/Kovalyovv/chat-app/internal/usecase"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

const serviceName = "chat-app"

func main() {
	tp, err := observability.InitTracer(serviceName, "jaeger:4317")
	if err != nil {
		slog.Error("failed to initialize tracer", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			slog.Error("failed to shutdown tracer", "error", err)
		}
	}()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
	cfg := config.NewFromEnv()

	if cfg.Database.URL == "" {
		slog.Error("missing critical configuration: DB_URL must be set")
		os.Exit(1)
	}
	if cfg.AuthService.Addr == "" {
		slog.Error("missing critical configuration: AUTH_SERVICE_ADDR must be set")
		os.Exit(1)
	}
	if cfg.InternalAPIKey == "" {
		slog.Error("missing critical configuration: INTERNAL_API_KEY must be set")
		os.Exit(1)
	}

	store, err := postgres.New(cfg.Database.URL)
	if err != nil {
		slog.Error("db connection failed", "error", err)
		os.Exit(1)
	}
	defer store.Close()

	roomUC := usecase.NewRoomUseCase(store.RoomRepo)
	messageUC := usecase.NewMessageUseCase(store.MessageRepo, store.ChatStateRepo)
	systemMessages := make(chan *domain.Message, 100)
	eventBus := bus.New(systemMessages)
	hub := ws.NewHub(messageUC, systemMessages, logger.With("component", "hub"))

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go hub.Run(ctx)

	authMW, authConn := middleware.NewAuthMiddleware(cfg.AuthService.Addr, logger)
	if authConn != nil {
		defer authConn.Close()
	}

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(otelgin.Middleware(serviceName))
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	httpDelivery.SetupRoutes(router, cfg, roomUC, messageUC, authMW, logger, hub, eventBus)

	srv := &http.Server{
		Addr:    ":" + cfg.API.Port,
		Handler: router,
	}

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

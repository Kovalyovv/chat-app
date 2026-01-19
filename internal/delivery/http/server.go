package http

import (
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Kovalyovv/chat-app/internal/bus"
	conf "github.com/Kovalyovv/chat-app/internal/config"
	"github.com/Kovalyovv/chat-app/internal/delivery/ws"
	"github.com/Kovalyovv/chat-app/internal/middleware"
	"github.com/Kovalyovv/chat-app/internal/usecase"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func SetupRoutes(
	router *gin.Engine,
	cfg *conf.Config,
	roomUC *usecase.RoomUseCase,
	messageUC *usecase.MessageUseCase,
	authMW gin.HandlerFunc,
	logger *slog.Logger,
	hub *ws.Hub,
	eventBus *bus.EventBus,
) {
	config := cors.DefaultConfig()
	config.AllowCredentials = true
	config.AllowOriginFunc = func(origin string) bool {
		u, err := url.Parse(origin)
		if err != nil {
			return false
		}
		hostname := strings.Trim(u.Hostname(), "[]")
		return (hostname == "localhost" || hostname == "0.0.0.0" || hostname == "127.0.0.1" || hostname == "::1") && u.Port() == "9002"
	}
	config.AllowHeaders = []string{"Origin", "Content-Type", "Authorization"}
	config.AllowMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}
	config.MaxAge = 12 * time.Hour

	router.Use(cors.New(config))

	router.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "chat-service running"})
	})

	api := router.Group("/api/v1")

	{
		rh := NewRoomHandler(roomUC, logger.With("handler", "room"))
		rooms := api.Group("/rooms")
		rooms.Use(authMW)
		rooms.POST("", rh.CreateRoom)
		rooms.POST("/join", rh.JoinRoom)
		rooms.GET("", rh.GetRooms)
	}

	{
		internal := api.Group("/internal")
		internal.Use(middleware.InternalAuthMiddleware(cfg.InternalAPIKey))

		notifyHandler := NewNotificationHandler(messageUC, eventBus, logger.With("handler", "notification"))
		internal.POST("/notify", notifyHandler.SendNotification)
	}

	{
		wsHandler := ws.NewWSHandler(hub, roomUC, messageUC, cfg.HistoryLimit, logger.With("handler", "ws"))
		wsGroup := api.Group("/ws")
		wsGroup.Use(authMW)
		wsGroup.GET("/:roomId", wsHandler.Handle)
	}
}

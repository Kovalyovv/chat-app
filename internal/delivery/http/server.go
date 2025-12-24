package http

import (
	"fmt"
	"net/http"

	"github.com/Kovalyovv/chat-app/internal/config"
	"github.com/Kovalyovv/chat-app/internal/delivery/ws"
	"github.com/Kovalyovv/chat-app/internal/logger"
	"github.com/Kovalyovv/chat-app/internal/usecase"
	"github.com/gin-gonic/gin"
)

func NewServer(
	cfg *config.Config,
	roomUC *usecase.RoomUseCase,
	messageUC *usecase.MessageUseCase,
	authMW gin.HandlerFunc,
	l *logger.Logger,
	hub *ws.Hub,
) *http.Server {
	router := gin.New()
	router.Use(gin.Recovery())

	router.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "chat-service running"})
	})

	api := router.Group("/api/v1")

	{
		rh := NewRoomHandler(roomUC, l)
		rooms := api.Group("/rooms")
		rooms.Use(authMW)
		rooms.POST("", rh.CreateRoom)
		rooms.POST("/join", rh.JoinRoom)
		rooms.GET("", rh.GetRooms)
	}

	{
		wsHandler := ws.NewWSHandler(hub, roomUC, messageUC)

		wsGroup := api.Group("/ws")
		wsGroup.Use(authMW)
		wsGroup.GET("/:roomId", wsHandler.Handle)
	}

	addr := fmt.Sprintf(":%s", cfg.API.Port)
	return &http.Server{Addr: addr, Handler: router}
}

package http

import (
	"log/slog"
	"net/http"

	"github.com/Kovalyovv/chat-app/internal/delivery/ws"
	"github.com/gin-gonic/gin"
)

type NotificationHandler struct {
	hub *ws.Hub
	log *slog.Logger
}

func NewNotificationHandler(hub *ws.Hub, logger *slog.Logger) *NotificationHandler {
	return &NotificationHandler{hub: hub, log: logger}
}

type notificationPayload struct {
	RoomID     int64  `json:"room_id" binding:"required"`
	UploaderID int64  `json:"uploader_id" binding:"required"`
	ObjectKey  string `json:"object_key" binding:"required"`
}

func (h *NotificationHandler) SendNotification(c *gin.Context) {
	var payload notificationPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		h.log.Warn("failed to bind notification payload", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	h.hub.BroadcastSystemMessage(payload.RoomID, payload.UploaderID, payload.ObjectKey)
	h.log.Info("broadcasted system notification", "room_id", payload.RoomID)
	c.Status(http.StatusOK)
}

package http

import (
	"log/slog"
	"net/http"

	"github.com/Kovalyovv/chat-app/internal/bus"
	"github.com/Kovalyovv/chat-app/internal/domain"
	"github.com/Kovalyovv/chat-app/internal/usecase"
	"github.com/gin-gonic/gin"
)

type NotificationHandler struct {
	messageUC *usecase.MessageUseCase
	eventBus  *bus.EventBus
	log       *slog.Logger
}

func NewNotificationHandler(uc *usecase.MessageUseCase, bus *bus.EventBus, logger *slog.Logger) *NotificationHandler {
	return &NotificationHandler{messageUC: uc, eventBus: bus, log: logger}
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

	msg := &domain.Message{
		RoomID: payload.RoomID,
		UserID: payload.UploaderID,
		Type:   "FILE",
		Metadata: map[string]interface{}{
			"object_key": payload.ObjectKey,
		},
	}

	msgID, err := h.messageUC.Save(c.Request.Context(), msg)
	if err != nil {
		h.log.Error("failed to save system message", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not process notification"})
		return
	}
	msg.ID = msgID

	h.eventBus.Publish(msg)

	h.log.Info("broadcasted system notification", "room_id", payload.RoomID)
	c.Status(http.StatusOK)
}

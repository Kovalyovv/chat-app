package ws

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type WSHandler struct {
	Hub          *Hub
	RoomUC       HandlerRoomUseCase
	MessageUC    HandlerMessageUseCase
	historyLimit int
	log          *slog.Logger
}

func NewWSHandler(hub *Hub, roomUC HandlerRoomUseCase, messageUC HandlerMessageUseCase, historyLimit int, logger *slog.Logger) *WSHandler {
	return &WSHandler{
		Hub:          hub,
		RoomUC:       roomUC,
		MessageUC:    messageUC,
		historyLimit: historyLimit,
		log:          logger,
	}
}

func (h *WSHandler) Handle(c *gin.Context) {
	userID := c.GetInt64("userID")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	roomID, err := strconv.ParseInt(c.Param("roomId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid room id"})
		return
	}

	ok, err := h.RoomUC.IsUserInRoom(c.Request.Context(), userID, roomID)
	if err != nil {
		h.log.Error("membership check failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "membership check failed"})
		return
	}
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "not a room member"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.log.Error("websocket upgrade failed", "error", err)
		return
	}

	client := NewClient(userID, roomID, conn, h.Hub, h.log.With("user_id", userID, "room_id", roomID))

	h.Hub.Register(client)

	go client.WritePump()

	go h.sendHistory(c.Request.Context(), client, roomID)

	client.ReadPump(c.Request.Context())
}

func (h *WSHandler) sendHistory(ctx context.Context, client *Client, roomID int64) {
	history, err := h.MessageUC.History(ctx, roomID, h.historyLimit)
	if err != nil {
		h.log.Error("failed to fetch message history", "room_id", roomID, "error", err)
		return
	}

	for i := len(history) - 1; i >= 0; i-- {
		msg := history[i]
		select {
		case client.Send <- OutgoingMessage{
			Type:      MsgHistory,
			UserID:    msg.UserID,
			MessageID: msg.ID,
			RoomID:    roomID,
			Payload:   msg,
			Timestamp: msg.CreatedAt.Unix(),
		}:
		case <-ctx.Done():
			h.log.Warn("client disconnected while sending history", "room_id", roomID)
			return
		}
	}
}

package ws

import (
	"net/http"
	"strconv"

	"github.com/Kovalyovv/chat-app/internal/usecase"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // позже ограничим
	},
}

type WSHandler struct {
	Hub       *Hub
	RoomUC    *usecase.RoomUseCase
	MessageUC *usecase.MessageUseCase
}

func NewWSHandler(hub *Hub, roomUC *usecase.RoomUseCase, messageUC *usecase.MessageUseCase) *WSHandler {
	return &WSHandler{
		Hub:       hub,
		RoomUC:    roomUC,
		MessageUC: messageUC,
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "membership check failed"})
		return
	}
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "not a room member"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}

	client := NewClient(userID, roomID, conn, h.Hub)

	h.Hub.Register(client)
	history, err := h.MessageUC.History(c.Request.Context(), roomID, 50)
	if err == nil {
		for i := len(history) - 1; i >= 0; i-- {
			client.Send <- OutgoingMessage{
				Type:      "history",
				UserID:    history[i].UserID,
				RoomID:    roomID,
				Payload:   history[i].Text,
				Timestamp: history[i].CreatedAt,
			}
		}
	}
	h.Hub.BroadcastSystem(roomID, MsgJoin, userID)

	go client.WritePump()
	client.ReadPump()
}

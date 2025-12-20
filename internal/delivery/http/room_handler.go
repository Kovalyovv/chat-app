package http

import (
	"net/http"

	"github.com/Kovalyovv/chat-app/internal/logger"
	"github.com/Kovalyovv/chat-app/internal/usecase"
	"github.com/gin-gonic/gin"
)

type RoomHandler struct {
	uc  *usecase.RoomUseCase
	log *logger.Logger
}

func NewRoomHandler(uc *usecase.RoomUseCase, l *logger.Logger) *RoomHandler {
	return &RoomHandler{uc: uc, log: l}
}

type createRoomReq struct {
	Name string `json:"name" binding:"required,min=3,max=50"`
}

type joinRoomReq struct {
	RoomID     int64  `json:"room_id" binding:"required"`
	InviteCode string `json:"invite_code" binding:"required"`
}

func (h *RoomHandler) CreateRoom(c *gin.Context) {
	var req createRoomReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userIDIface, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found in context"})
		return
	}
	userID := userIDIface.(int64)

	room, err := h.uc.CreateRoom(c.Request.Context(), req.Name, userID)
	if err != nil {
		h.log.Infof("create room error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create room " + err.Error()})
		return
	}
	c.JSON(http.StatusCreated, room)
}

func (h *RoomHandler) JoinRoom(c *gin.Context) {
	var req joinRoomReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userIDIface, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found in context"})
		return
	}
	userID := userIDIface.(int64)

	room, err := h.uc.JoinRoom(c.Request.Context(), userID, req.RoomID, req.InviteCode)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, room)
}

func (h *RoomHandler) GetRooms(c *gin.Context) {
	userIDIface, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found in context"})
		return
	}
	userID := userIDIface.(int64)

	rooms, err := h.uc.GetRoomsByUser(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load rooms"})
		return
	}
	c.JSON(http.StatusOK, rooms)
}

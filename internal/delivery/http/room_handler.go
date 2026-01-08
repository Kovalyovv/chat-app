package http

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/Kovalyovv/chat-app/internal/domain"
	"github.com/gin-gonic/gin"
)

type RoomUseCase interface {
	CreateRoom(ctx context.Context, name string, ownerID int64) (*domain.Room, error)
	JoinRoom(ctx context.Context, userID, roomID int64, inviteCode string) (*domain.Room, error)
	GetRoomsByUser(ctx context.Context, userID int64) ([]domain.Room, error)
}

type RoomHandler struct {
	uc  RoomUseCase
	log *slog.Logger
}

func NewRoomHandler(uc RoomUseCase, l *slog.Logger) *RoomHandler {
	return &RoomHandler{uc: uc, log: l}
}

type apiError struct {
	Error string `json:"error"`
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
		c.JSON(http.StatusBadRequest, apiError{Error: "invalid request payload"})
		return
	}

	userID := c.GetInt64("userID")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, apiError{Error: "user not found in context"})
		return
	}

	room, err := h.uc.CreateRoom(c.Request.Context(), req.Name, userID)
	if err != nil {
		h.log.Error("create room error", "error", err)
		c.JSON(http.StatusInternalServerError, apiError{Error: "failed to create room"})
		return
	}
	c.JSON(http.StatusCreated, room)
}

func (h *RoomHandler) JoinRoom(c *gin.Context) {
	var req joinRoomReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiError{Error: "invalid request payload"})
		return
	}

	userID := c.GetInt64("userID")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, apiError{Error: "user not found in context"})
		return
	}

	_, err := h.uc.JoinRoom(c.Request.Context(), userID, req.RoomID, req.InviteCode)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidInviteCode) {
			c.JSON(http.StatusForbidden, apiError{Error: err.Error()})
			return
		}
		h.log.Error("join room error", "error", err)
		c.JSON(http.StatusInternalServerError, apiError{Error: "failed to join room"})
		return
	}

	c.Status(http.StatusOK)
}

func (h *RoomHandler) GetRooms(c *gin.Context) {
	userID := c.GetInt64("userID")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, apiError{Error: "user not found in context"})
		return
	}

	rooms, err := h.uc.GetRoomsByUser(c.Request.Context(), userID)
	if err != nil {
		h.log.Error("failed to load rooms", "error", err)
		c.JSON(http.StatusInternalServerError, apiError{Error: "failed to load rooms"})
		return
	}
	c.JSON(http.StatusOK, rooms)
}

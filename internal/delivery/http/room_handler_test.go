package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Kovalyovv/chat-app/internal/domain"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// --- Mock Usecase ---
type MockRoomUseCase struct {
	mock.Mock
}

func (m *MockRoomUseCase) CreateRoom(ctx context.Context, name string, ownerID int64) (*domain.Room, error) {
	args := m.Called(ctx, name, ownerID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Room), args.Error(1)
}

func (m *MockRoomUseCase) JoinRoom(ctx context.Context, userID int64, roomID int64, inviteCode string) (*domain.Room, error) {
	args := m.Called(ctx, userID, roomID, inviteCode)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Room), args.Error(1)
}

func (m *MockRoomUseCase) GetRoomsByUser(ctx context.Context, userID int64) ([]domain.Room, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.Room), args.Error(1)
}

func TestRoomHandler_CreateRoom(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	t.Run("Given a valid request", func(t *testing.T) {
		// --- GIVEN ---
		mockUC := new(MockRoomUseCase)
		handler := NewRoomHandler(mockUC, logger)

		router := gin.New()
		// Middleware to simulate a logged-in user
		router.Use(func(c *gin.Context) {
			c.Set("userID", int64(1))
			c.Next()
		})
		router.POST("/rooms", handler.CreateRoom)

		roomName := "My New Room"
		expectedRoom := &domain.Room{ID: 1, Name: roomName, OwnerID: 1}

		mockUC.On("CreateRoom", mock.Anything, roomName, int64(1)).Return(expectedRoom, nil).Once()

		body, _ := json.Marshal(createRoomReq{Name: roomName})
		req, _ := http.NewRequest(http.MethodPost, "/rooms", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		// --- WHEN ---
		router.ServeHTTP(rr, req)

		// --- THEN ---
		assert.Equal(t, http.StatusCreated, rr.Code)

		var returnedRoom domain.Room
		json.Unmarshal(rr.Body.Bytes(), &returnedRoom)
		assert.Equal(t, expectedRoom.Name, returnedRoom.Name)

		mockUC.AssertExpectations(t)
	})

	t.Run("Given usecase returns an error", func(t *testing.T) {
		// --- GIVEN ---
		mockUC := new(MockRoomUseCase)
		handler := NewRoomHandler(mockUC, logger)
		router := gin.New()
		router.Use(func(c *gin.Context) { c.Set("userID", int64(1)); c.Next() })
		router.POST("/rooms", handler.CreateRoom)

		ucError := errors.New("usecase failed")
		mockUC.On("CreateRoom", mock.Anything, mock.Anything, mock.Anything).Return(nil, ucError).Once()

		body, _ := json.Marshal(createRoomReq{Name: "Fail Room"})
		req, _ := http.NewRequest(http.MethodPost, "/rooms", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		// --- WHEN ---
		router.ServeHTTP(rr, req)

		// --- THEN ---
		assert.Equal(t, http.StatusInternalServerError, rr.Code)

		var errResp apiError
		json.Unmarshal(rr.Body.Bytes(), &errResp)
		assert.Equal(t, "failed to create room", errResp.Error)

		mockUC.AssertExpectations(t)
	})
}

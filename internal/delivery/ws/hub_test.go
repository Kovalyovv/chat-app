package ws

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/Kovalyovv/chat-app/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type MockMessageUseCase struct {
	mock.Mock
}

func (m *MockMessageUseCase) Save(ctx context.Context, msg *domain.Message) (int64, error) {
	args := m.Called(ctx, msg)
	return int64(args.Int(0)), args.Error(1)
}

func (m *MockMessageUseCase) UpdateDeliveryState(ctx context.Context, roomID, userID, messageID int64) error {
	args := m.Called(ctx, roomID, userID, messageID)
	return args.Error(0)
}

func (m *MockMessageUseCase) UpdateReadState(ctx context.Context, roomID, userID, lastReadID int64) error {
	args := m.Called(ctx, roomID, userID, lastReadID)
	return args.Error(0)
}

func (m *MockMessageUseCase) GetChatState(ctx context.Context, roomID, userID int64) (domain.ChatState, error) {
	args := m.Called(ctx, roomID, userID)
	return args.Get(0).(domain.ChatState), args.Error(1)
}

func (m *MockMessageUseCase) GetAfter(ctx context.Context, roomID, afterID int64, limit int) ([]domain.Message, error) {
	args := m.Called(ctx, roomID, afterID, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.Message), args.Error(1)
}

type mockClient struct {
	UserID int64
	RoomID int64
	Send   chan OutgoingMessage
}

func newMockClient(userID, roomID int64) *mockClient {
	return &mockClient{
		UserID: userID,
		RoomID: roomID,
		Send:   make(chan OutgoingMessage, 10),
	}
}

func TestHub_Broadcast(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	mockUC := new(MockMessageUseCase) // This is now a *MockMessageUseCase
	systemMessages := make(chan *domain.Message)

	hub := NewHub(mockUC, systemMessages, logger)
	go hub.Run(context.Background())

	t.Run("Given a message from one user", func(t *testing.T) {
		// --- GIVEN ---
		roomID := int64(101)
		clientA := newMockClient(1, roomID)
		clientB := newMockClient(2, roomID)

		realClientA := &Client{UserID: clientA.UserID, RoomID: clientA.RoomID, Send: clientA.Send}
		realClientB := &Client{UserID: clientB.UserID, RoomID: clientB.RoomID, Send: clientB.Send}

		hub.Register(realClientA)
		hub.Register(realClientB)

		event := Event{
			Type:    EventMessage,
			RoomID:  roomID,
			UserID:  clientA.UserID,
			Text:    "Hello",
			Client:  realClientA,
			Context: context.Background(),
		}

		expectedMsgID := int64(123)
		mockUC.On("Save", mock.Anything, mock.AnythingOfType("*domain.Message")).Return(int(expectedMsgID), nil).Once()

		mockUC.On("UpdateDeliveryState", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

		// --- WHEN ---
		hub.Events <- event

		// --- THEN ---
		select {
		case msg := <-clientB.Send:
			assert.Equal(t, MsgMessage, msg.Type)
			assert.Equal(t, expectedMsgID, msg.MessageID)
			payload, ok := msg.Payload.(*domain.Message)
			require.True(t, ok)
			assert.Equal(t, "Hello", payload.Text)
		case <-time.After(100 * time.Millisecond):
			t.Fatal("Client B did not receive message in time")
		}

		select {
		case msg := <-clientA.Send:
			assert.Equal(t, MsgMessage, msg.Type)
			assert.Equal(t, expectedMsgID, msg.MessageID)
		case <-time.After(100 * time.Millisecond):
			t.Fatal("Client A did not receive message in time")
		}

		mockUC.AssertExpectations(t)
	})
}

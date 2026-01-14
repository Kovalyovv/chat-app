package usecase

import (
	"context"
	"errors"
	"regexp"
	"testing"

	"github.com/Kovalyovv/chat-app/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type MockRoomRepository struct {
	mock.Mock
}

func (m *MockRoomRepository) Create(ctx context.Context, name string, ownerID int64, inviteCode string) (*domain.Room, error) {
	args := m.Called(ctx, name, ownerID, inviteCode)
	ret := args.Get(0)
	if ret == nil {
		return nil, args.Error(1)
	}

	var room *domain.Room
	if fn, ok := ret.(func(context.Context, string, int64, string) *domain.Room); ok {
		room = fn(ctx, name, ownerID, inviteCode)
	} else {
		room = ret.(*domain.Room)
	}

	return room, args.Error(1)
}

func (m *MockRoomRepository) AddMember(ctx context.Context, userID, roomID int64) error {
	args := m.Called(ctx, userID, roomID)
	return args.Error(0)
}

func (m *MockRoomRepository) FindByID(ctx context.Context, id int64) (*domain.Room, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Room), args.Error(1)
}

func (m *MockRoomRepository) GetByUserID(ctx context.Context, userID int64) ([]domain.Room, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.Room), args.Error(1)
}

func (m *MockRoomRepository) IsUserInRoom(ctx context.Context, userID, roomID int64) (bool, error) {
	args := m.Called(ctx, userID, roomID)
	return args.Bool(0), args.Error(1)
}

func TestRoomUseCase_CreateRoom(t *testing.T) {
	mockRepo := new(MockRoomRepository)
	uc := NewRoomUseCase(mockRepo)
	ctx := context.Background()

	t.Run("Given successful creation", func(t *testing.T) {
		// --- GIVEN ---
		roomName := "Test Room"
		ownerID := int64(1)
		expectedRoom := &domain.Room{ID: 123, Name: roomName, OwnerID: ownerID}

		mockRepo.On("Create", ctx, roomName, ownerID, mock.AnythingOfType("string")).
			Return(func(ctx context.Context, name string, ownerID int64, inviteCode string) *domain.Room {
				return &domain.Room{ID: 123, Name: name, OwnerID: ownerID, InviteCode: inviteCode}
			}, nil).Once()
		mockRepo.On("AddMember", ctx, ownerID, expectedRoom.ID).Return(nil).Once()

		// --- WHEN ---
		room, err := uc.CreateRoom(ctx, roomName, ownerID)

		// --- THEN ---
		require.NoError(t, err)
		assert.Equal(t, int64(123), room.ID)
		assert.Equal(t, roomName, room.Name)
		assert.Regexp(t, regexp.MustCompile(`^[a-z]{3}-\d{4}-[a-z]{3}$`), room.InviteCode)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Given repository Create fails", func(t *testing.T) {
		// --- GIVEN ---
		repoErr := errors.New("database error")
		mockRepo.On("Create", ctx, mock.Anything, mock.Anything, mock.Anything).Return(nil, repoErr).Once()

		// --- WHEN ---
		_, err := uc.CreateRoom(ctx, "Fail Room", 1)

		// --- THEN ---
		assert.ErrorIs(t, err, repoErr)
		mockRepo.AssertExpectations(t)
	})
}

func TestRoomUseCase_JoinRoom(t *testing.T) {
	mockRepo := new(MockRoomRepository)
	uc := NewRoomUseCase(mockRepo)
	ctx := context.Background()

	t.Run("Given valid room and invite code", func(t *testing.T) {
		// --- GIVEN ---
		userID, roomID := int64(2), int64(1)
		inviteCode := "abc-1234-def"
		expectedRoom := &domain.Room{ID: roomID, InviteCode: inviteCode}

		mockRepo.On("FindByID", ctx, roomID).Return(expectedRoom, nil).Once()
		mockRepo.On("AddMember", ctx, userID, roomID).Return(nil).Once()

		// --- WHEN ---
		room, err := uc.JoinRoom(ctx, userID, roomID, inviteCode)

		// --- THEN ---
		require.NoError(t, err)
		assert.Equal(t, expectedRoom, room)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Given incorrect invite code", func(t *testing.T) {
		// --- GIVEN ---
		userID, roomID := int64(2), int64(1)
		expectedRoom := &domain.Room{ID: roomID, InviteCode: "correct-code"}

		mockRepo.On("FindByID", ctx, roomID).Return(expectedRoom, nil).Once()

		// --- WHEN ---
		_, err := uc.JoinRoom(ctx, userID, roomID, "wrong-code")

		// --- THEN ---
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid invite code")
		mockRepo.AssertExpectations(t)
	})
}

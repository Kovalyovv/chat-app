package usecase

import (
	"context"

	"github.com/Kovalyovv/chat-app/internal/domain"
)

type RoomRepository interface {
	Create(ctx context.Context, name string, ownerID int64, inviteCode string) (*domain.Room, error)
	AddMember(ctx context.Context, userID, roomID int64) error
	FindByID(ctx context.Context, id int64) (*domain.Room, error)
	GetByUserID(ctx context.Context, userID int64) ([]domain.Room, error)
	IsUserInRoom(ctx context.Context, userID, roomID int64) (bool, error)
}

type MessageRepository interface {
	Save(ctx context.Context, msg *domain.Message) (int64, error)
	GetLast(ctx context.Context, roomID int64, limit int) ([]domain.Message, error)
	GetAfter(ctx context.Context, roomID int64, afterID int64, limit int) ([]domain.Message, error)
}

type ChatStateRepository interface {
	UpsertDelivery(ctx context.Context, roomID, userID, messageID int64) error
	UpsertRead(ctx context.Context, roomID, userID, lastReadID int64) error
	GetState(ctx context.Context, roomID, userID int64) (domain.ChatState, error)
}

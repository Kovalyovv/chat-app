package ws

import (
	"context"

	"github.com/Kovalyovv/chat-app/internal/domain"
)

type HubMessageUseCase interface {
	Save(ctx context.Context, msg *domain.Message) (int64, error)
	UpdateDeliveryState(ctx context.Context, roomID, userID, messageID int64) error
	UpdateReadState(ctx context.Context, roomID, userID, lastReadID int64) error
	GetChatState(ctx context.Context, roomID, userID int64) (domain.ChatState, error)
	GetAfter(ctx context.Context, roomID, afterID int64, limit int) ([]domain.Message, error)
}

type HandlerMessageUseCase interface {
	History(ctx context.Context, roomID int64, limit int) ([]domain.Message, error)
}

type HandlerRoomUseCase interface {
	IsUserInRoom(ctx context.Context, userID, roomID int64) (bool, error)
}

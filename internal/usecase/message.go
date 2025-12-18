package usecase

import (
	"context"

	"github.com/Kovalyovv/chat-app/internal/domain"
	"github.com/Kovalyovv/chat-app/internal/repository/postgres"
)

type MessageUseCase struct {
	messageRepo   *postgres.MessageRepo
	chatStateRepo *postgres.ChatStateRepo
}

func NewMessageUseCase(store *postgres.Store) *MessageUseCase {
	return &MessageUseCase{messageRepo: store.MessageRepo, chatStateRepo: store.ChatStateRepo}
}

func (uc *MessageUseCase) Save(
	ctx context.Context,
	roomID, userID int64,
	text string,
) (int64, error) {
	return uc.messageRepo.Save(ctx, roomID, userID, text)
}

func (uc *MessageUseCase) History(ctx context.Context, roomID int64, limit int) ([]domain.Message, error) {
	return uc.messageRepo.GetLast(ctx, roomID, limit)
}

func (uc *MessageUseCase) GetAfter(
	ctx context.Context,
	roomID int64,
	afterID int64,
	limit int,
) ([]domain.Message, error) {
	return uc.messageRepo.GetAfter(ctx, roomID, afterID, limit)
}

func (uc *MessageUseCase) GetChatState(
	ctx context.Context,
	roomID, userID int64,
) (domain.ChatState, error) {
	return uc.chatStateRepo.GetState(ctx, roomID, userID)
}

func (uc *MessageUseCase) UpdateDeliveryState(
	ctx context.Context,
	roomID, userID int64,
	messageID int64,
) error {
	return uc.chatStateRepo.UpsertDelivery(ctx, roomID, userID, messageID)
}

func (uc *MessageUseCase) UpdateReadState(
	ctx context.Context,
	roomID, userID int64,
	lastReadID int64,
) error {
	return uc.chatStateRepo.UpsertRead(ctx, roomID, userID, lastReadID)
}

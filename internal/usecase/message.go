package usecase

import (
	"context"

	"github.com/Kovalyovv/chat-app/internal/domain"
)

type MessageUseCase struct {
	messageRepo   MessageRepository
	chatStateRepo ChatStateRepository
}

func NewMessageUseCase(mRepo MessageRepository, sRepo ChatStateRepository) *MessageUseCase {
	return &MessageUseCase{messageRepo: mRepo, chatStateRepo: sRepo}
}

func (uc *MessageUseCase) Save(ctx context.Context, msg *domain.Message) (int64, error) {
	return uc.messageRepo.Save(ctx, msg)
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

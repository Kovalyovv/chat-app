package usecase

import (
	"context"

	"github.com/Kovalyovv/chat-app/internal/domain"
	"github.com/Kovalyovv/chat-app/internal/repository/postgres"
)

type MessageUseCase struct {
	messageRepo   *postgres.MessageRepo
	readStateRepo *postgres.ReadStateRepo
}

func NewMessageUseCase(store *postgres.Store) *MessageUseCase {
	return &MessageUseCase{messageRepo: store.MessageRepo}
}

func (uc *MessageUseCase) Save(
	ctx context.Context,
	roomID, userID int,
	text string,
) (int64, error) {
	return uc.messageRepo.Save(ctx, roomID, userID, text)
}

func (uc *MessageUseCase) History(ctx context.Context, roomID int, limit int) ([]domain.Message, error) {
	return uc.messageRepo.GetLast(ctx, roomID, limit)
}
func (uc *MessageUseCase) UpdateReadState(
	ctx context.Context,
	roomID, userID int,
	lastReadID int64,
) error {
	return uc.readStateRepo.Upsert(ctx, roomID, userID, lastReadID)
}

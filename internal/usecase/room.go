package usecase

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"strings"

	"github.com/Kovalyovv/chat-app/internal/domain"
	"github.com/Kovalyovv/chat-app/internal/repository/postgres"
)

const (
	letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	digitBytes  = "0123456789"
)

type RoomUseCase struct {
	repo *postgres.RoomRepo
}

func NewRoomUseCase(store *postgres.Store) *RoomUseCase {
	return &RoomUseCase{repo: store.RoomRepo}
}

func (uc *RoomUseCase) CreateRoom(ctx context.Context, name string, ownerID int64) (*domain.Room, error) {
	code, err := generateInviteCode()
	if err != nil {
		return nil, err
	}

	room, err := uc.repo.Create(ctx, name, ownerID, code)
	if err != nil {
		return nil, err
	}

	if err := uc.repo.AddMember(ctx, ownerID, room.ID); err != nil {
		return nil, err
	}

	return room, nil
}

func (uc *RoomUseCase) JoinRoom(ctx context.Context, userID int64, roomID int64, inviteCode string) (*domain.Room, error) {
	room, err := uc.repo.FindByID(ctx, roomID)
	if err != nil {
		return nil, err
	}
	if room.InviteCode != inviteCode {
		return nil, fmt.Errorf("invalid invite code")
	}
	if err := uc.repo.AddMember(ctx, userID, room.ID); err != nil {
		return nil, err
	}
	return room, nil
}

func (uc *RoomUseCase) GetRoomsByUser(ctx context.Context, userID int64) ([]domain.Room, error) {
	return uc.repo.GetByUserID(ctx, userID)
}

func (uc *RoomUseCase) IsUserInRoom(ctx context.Context, userID, roomID int64) (bool, error) {
	return uc.repo.IsUserInRoom(ctx, userID, roomID)
}

func generateInviteCode() (string, error) {
	part1, err := randomString(3, letterBytes)
	if err != nil {
		return "", err
	}
	part2, err := randomString(4, digitBytes)
	if err != nil {
		return "", err
	}
	part3, err := randomString(3, letterBytes)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s-%s-%s", strings.ToLower(part1), part2, strings.ToLower(part3)), nil
}

func randomString(length int, charSet string) (string, error) {
	b := make([]byte, length)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", err
	}
	for i := range b {
		b[i] = charSet[b[i]%byte(len(charSet))]
	}
	return string(b), nil
}

package postgres

import (
	"context"
	"fmt"

	"github.com/Kovalyovv/chat-app/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RoomRepo struct {
	pool *pgxpool.Pool
}

func NewRoomRepo(pool *pgxpool.Pool) *RoomRepo { return &RoomRepo{pool: pool} }

func (r *RoomRepo) Create(ctx context.Context, name string, ownerID int64, inviteCode string) (*domain.Room, error) {
	query := `INSERT INTO rooms (name, owner_id, invite_code) VALUES ($1, $2, $3)
              RETURNING id, name, owner_id, invite_code, created_at`
	var rm domain.Room
	err := r.pool.QueryRow(ctx, query, name, ownerID, inviteCode).Scan(
		&rm.ID, &rm.Name, &rm.OwnerID, &rm.InviteCode, &rm.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create room: %w", err)
	}
	return &rm, nil
}

func (r *RoomRepo) AddMember(ctx context.Context, userID, roomID int64) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO room_members (user_id, room_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, userID, roomID)
	return err
}

func (r *RoomRepo) FindByInviteCode(ctx context.Context, code string) (*domain.Room, error) {
	var rm domain.Room
	err := r.pool.QueryRow(ctx, `SELECT id, name, owner_id, invite_code, created_at FROM rooms WHERE invite_code=$1`, code).Scan(
		&rm.ID, &rm.Name, &rm.OwnerID, &rm.InviteCode, &rm.CreatedAt,
	)
	return &rm, err
}

func (r *RoomRepo) FindByID(ctx context.Context, id int64) (*domain.Room, error) {
	var rm domain.Room
	err := r.pool.QueryRow(ctx, `SELECT id, name, owner_id, invite_code, created_at FROM rooms WHERE id=$1`, id).Scan(
		&rm.ID, &rm.Name, &rm.OwnerID, &rm.InviteCode, &rm.CreatedAt,
	)
	return &rm, err
}

func (r *RoomRepo) GetByUserID(ctx context.Context, userID int64) ([]domain.Room, error) {
	rows, err := r.pool.Query(ctx, `SELECT r.id, r.name, r.owner_id, r.invite_code, r.created_at FROM rooms r JOIN room_members rm ON r.id = rm.room_id WHERE rm.user_id = $1`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []domain.Room
	for rows.Next() {
		var rm domain.Room
		if err := rows.Scan(&rm.ID, &rm.Name, &rm.OwnerID, &rm.InviteCode, &rm.CreatedAt); err != nil {
			return nil, err
		}
		res = append(res, rm)
	}
	return res, nil
}

func (r *RoomRepo) IsUserInRoom(ctx context.Context, userID, roomID int64) (bool, error) {
	var dummy int64
	err := r.pool.QueryRow(ctx,
		`SELECT 1 FROM room_members WHERE user_id=$1 AND room_id=$2`,
		userID, roomID,
	).Scan(&dummy)

	if err != nil {
		if err == pgx.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

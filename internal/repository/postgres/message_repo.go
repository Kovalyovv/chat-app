package postgres

import (
	"context"

	"github.com/Kovalyovv/chat-app/internal/domain"
	"github.com/jackc/pgx/v5/pgxpool"
)

type MessageRepo struct {
	pool *pgxpool.Pool
}

func NewMessageRepo(pool *pgxpool.Pool) *MessageRepo {
	return &MessageRepo{pool: pool}
}

func (r *MessageRepo) Save(
	ctx context.Context,
	roomID, userID int,
	text string,
) (int64, error) {

	var id int64
	err := r.pool.QueryRow(ctx,
		`INSERT INTO messages(room_id, user_id, text)
		 VALUES ($1, $2, $3)
		 RETURNING id`,
		roomID, userID, text,
	).Scan(&id)

	return id, err
}

func (r *MessageRepo) GetLast(ctx context.Context, roomID, limit int) ([]domain.Message, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, room_id, user_id, text, created_at
		 FROM messages
		 WHERE room_id=$1
		 ORDER BY id DESC
		 LIMIT $2`,
		roomID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []domain.Message
	for rows.Next() {
		var m domain.Message
		if err := rows.Scan(
			&m.ID,
			&m.RoomID,
			&m.UserID,
			&m.Text,
			&m.CreatedAt,
		); err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	return msgs, nil
}

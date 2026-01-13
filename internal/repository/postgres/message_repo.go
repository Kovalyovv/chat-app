package postgres

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/Kovalyovv/chat-app/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type MessageRepo struct {
	pool *pgxpool.Pool
}

func NewMessageRepo(pool *pgxpool.Pool) *MessageRepo {
	return &MessageRepo{pool: pool}
}

func (r *MessageRepo) Save(ctx context.Context, msg *domain.Message) (int64, error) {

	var id int64

	metadataBytes, err := json.Marshal(msg.Metadata)
	if err != nil {
		return 0, err
	}

	query := `
		INSERT INTO messages(room_id, user_id, message_type, text, metadata)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`

	err = r.pool.QueryRow(ctx, query, msg.RoomID, msg.UserID, msg.Type, msg.Text, metadataBytes).Scan(&id)
	return id, err
}

func (r *MessageRepo) GetLast(ctx context.Context, roomID int64, limit int) ([]domain.Message, error) {
	query := `
		SELECT id, room_id, user_id, message_type, text, metadata, created_at
		FROM messages
		WHERE room_id=$1
		ORDER BY id DESC
		LIMIT $2
	`
	rows, err := r.pool.Query(ctx, query, roomID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMessages(rows)
}

func (r *MessageRepo) GetAfter(ctx context.Context, roomID int64, afterID int64, limit int) ([]domain.Message, error) {
	query := `
		SELECT id, room_id, user_id, message_type, text, metadata, created_at
        FROM messages
        WHERE room_id = $1 AND id > $2
        ORDER BY id ASC
        LIMIT $3
    `
	rows, err := r.pool.Query(ctx, query, roomID, afterID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMessages(rows)
}

func scanMessages(rows pgx.Rows) ([]domain.Message, error) {
	var msgs []domain.Message
	for rows.Next() {
		var m domain.Message
		var metadataBytes []byte
		var text sql.NullString

		if err := rows.Scan(
			&m.ID, &m.RoomID, &m.UserID, &m.Type, &text, &metadataBytes, &m.CreatedAt,
		); err != nil {
			return nil, err
		}

		if text.Valid {
			m.Text = text.String
		}

		if len(metadataBytes) > 0 {
			if err := json.Unmarshal(metadataBytes, &m.Metadata); err != nil {
				return nil, err
			}
		}
		msgs = append(msgs, m)
	}
	return msgs, nil
}

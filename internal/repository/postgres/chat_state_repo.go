// internal/repository/postgres/chat_state_repo.go
package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ChatStateRepo struct {
	pool *pgxpool.Pool
}

func NewChatStateRepo(pool *pgxpool.Pool) *ChatStateRepo {
	return &ChatStateRepo{pool: pool}
}

func (r *ChatStateRepo) UpsertDelivery(
	ctx context.Context,
	roomID, userID int64,
	lastDeliveredID int64,
) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO room_read_states (room_id, user_id, last_delivered_message_id)
		VALUES ($1, $2, $3)
		ON CONFLICT (room_id, user_id)
		DO UPDATE SET
			last_delivered_message_id = GREATEST(room_read_states.last_delivered_message_id, EXCLUDED.last_delivered_message_id),
			updated_at = now()
	`, roomID, userID, lastDeliveredID)
	return err
}

func (r *ChatStateRepo) UpsertRead(
	ctx context.Context,
	roomID, userID int64,
	lastReadID int64,
) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO room_read_states (room_id, user_id, last_read_message_id)
		VALUES ($1, $2, $3)
		ON CONFLICT (room_id, user_id)
		DO UPDATE SET
			last_read_message_id = GREATEST(room_read_states.last_read_message_id, EXCLUDED.last_read_message_id),
			updated_at = now()
	`, roomID, userID, lastReadID)
	return err
}

package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ReadStateRepo struct {
	pool *pgxpool.Pool
}

func NewReadStateRepo(pool *pgxpool.Pool) *ReadStateRepo {
	return &ReadStateRepo{pool: pool}
}

func (r *ReadStateRepo) Upsert(
	ctx context.Context,
	roomID, userID int,
	lastReadMessageID int64,
) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO room_read_states (room_id, user_id, last_read_message_id)
		VALUES ($1, $2, $3)
		ON CONFLICT (room_id, user_id)
		DO UPDATE SET
			last_read_message_id = GREATEST(
				room_read_states.last_read_message_id,
				EXCLUDED.last_read_message_id
			),
			updated_at = now()
	`, roomID, userID, lastReadMessageID)

	return err
}

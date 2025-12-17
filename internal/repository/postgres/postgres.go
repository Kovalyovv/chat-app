package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	Pool          *pgxpool.Pool
	RoomRepo      *RoomRepo
	MessageRepo   *MessageRepo
	ReadStateRepo *ReadStateRepo
}

func New(databaseUrl string) (*Store, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseUrl)
	if err != nil {
		return nil, err
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}

	return &Store{
		Pool:          pool,
		RoomRepo:      NewRoomRepo(pool),
		MessageRepo:   NewMessageRepo(pool),
		ReadStateRepo: NewReadStateRepo(pool),
	}, nil
}

func (s *Store) Close() { s.Pool.Close() }

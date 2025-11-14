// internal/store/postgres.go

package store

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
)

// Store определяет интерфейс для всех операций с базой данных.
// Пока он пустой, мы будем добавлять методы по мере необходимости.
type Store struct {
	pool *pgxpool.Pool
}

func New(databaseUrl string) (*Store, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseUrl)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to database: %w", err)
	}

	store := &Store{
		pool: pool,
	}

	return store, nil
}

// Close закрывает пул соединений с базой данных.
func (s *Store) Close() {
	s.pool.Close()
}

// Ping проверяет, что соединение с базой данных все еще живо.
func (s *Store) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

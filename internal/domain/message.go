package domain

import "time"

type Message struct {
	ID        int64
	RoomID    int64
	UserID    int64
	Text      string
	CreatedAt time.Time
}

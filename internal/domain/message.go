package domain

import "time"

type Message struct {
	ID        int
	RoomID    int
	UserID    int
	Text      string
	CreatedAt time.Time
}

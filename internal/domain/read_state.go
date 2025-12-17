package domain

import "time"

type ReadState struct {
	RoomID            int
	UserID            int
	LastReadMessageID int64
	UpdatedAt         time.Time
}

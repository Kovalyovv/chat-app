package domain

import "time"

type ChatState struct {
	RoomID                 int64     `json:"-"`
	UserID                 int64     `json:"-"`
	LastReadMessageID      int64     `json:"last_read_message_id,omitempty"`
	LastDeliveredMessageID int64     `json:"last_delivered_message_id,omitempty"`
	UpdatedAt              time.Time `json:"-"`
}

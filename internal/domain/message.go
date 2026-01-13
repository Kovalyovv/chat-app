package domain

import "time"

type Message struct {
	ID        int64                  `json:"id"`
	RoomID    int64                  `json:"room_id"`
	UserID    int64                  `json:"user_id"`
	Type      string                 `json:"type"`
	Text      string                 `json:"text,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
}

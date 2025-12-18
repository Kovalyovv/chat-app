package ws

import "time"

type IncomingMessage struct {
	Type    MessageType    `json:"type"`
	Payload map[string]any `json:"payload"`
}

type OutgoingMessage struct {
	Type        MessageType `json:"type"`
	MessageID   int64       `json:"message_id,omitempty"`
	ClientMsgID string      `json:"client_msg_id,omitempty"`
	UserID      int64       `json:"user_id,omitempty"`
	RoomID      int64       `json:"room_id,omitempty"`
	Payload     string      `json:"payload,omitempty"`
	Timestamp   time.Time   `json:"timestamp,omitempty"`
}

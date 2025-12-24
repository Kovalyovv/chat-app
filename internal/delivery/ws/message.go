package ws

import (
	"encoding/json"
	"errors"
)

// ===================== incoming =====================

type IncomingMessage struct {
	Type    MessageType     `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// Client -> Server payloads

type SendPayload struct {
	Text        string `json:"text"`
	ClientMsgID string `json:"client_msg_id"`
}

type ReadPayload struct {
	UpToMessageID int64 `json:"up_to_message_id"`
}

type ResyncPayload struct {
	LastRecvMessageID int64 `json:"last_recv_message_id"`
	ReadUpToMessageID int64 `json:"read_up_to_message_id"`
}

// ===================== outgoing =====================

type OutgoingMessage struct {
	Type        MessageType `json:"type"`
	RoomID      int64       `json:"room_id,omitempty"`
	UserID      int64       `json:"user_id,omitempty"`
	MessageID   int64       `json:"message_id,omitempty"`
	ClientMsgID string      `json:"client_msg_id,omitempty"`
	Payload     any         `json:"payload,omitempty"`
	Timestamp   int64       `json:"ts"`
}

// ===================== decoding helpers =====================

func DecodeIncoming(data []byte) (IncomingMessage, error) {
	var msg IncomingMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return msg, err
	}

	if msg.Type == 0 {
		return msg, errors.New("missing message type")
	}

	return msg, nil
}

func DecodePayload[T any](msg IncomingMessage) (T, error) {
	var p T
	if len(msg.Payload) == 0 {
		return p, errors.New("empty payload")
	}
	return p, json.Unmarshal(msg.Payload, &p)
}

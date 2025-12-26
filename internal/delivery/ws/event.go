package ws

import "context"

type EventType uint8

const (
	EventUnknown EventType = iota
	EventMessage
	EventJoin
	EventLeave
	EventRead
	EventResync
)

type Event struct {
	Type        EventType
	RoomID      int64
	UserID      int64
	Text        string
	ClientMsgID string
	Client      *Client
	ReadUpToID  int64
	LastRecvID  int64
	Context     context.Context
}

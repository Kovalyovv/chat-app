package ws

type EventType uint8

const (
	EventUnknown EventType = iota
	EventMessage
	EventJoin
	EventLeave
	EventRead
	EventResync
)

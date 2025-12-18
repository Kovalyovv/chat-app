package ws

type MessageType uint8

const (
	MsgUnknown MessageType = iota
	MsgMessage
	MsgJoin
	MsgLeave
	MsgAck
	MsgDelivered
	MsgRead
	MsgResync
	MsgStateUpdate
	MsgHistory
)

func (t MessageType) String() string {
	switch t {
	case MsgMessage:
		return "message"
	case MsgJoin:
		return "join"
	case MsgLeave:
		return "leave"
	case MsgAck:
		return "ack"
	case MsgDelivered:
		return "delivered"
	case MsgRead:
		return "read"
	case MsgResync:
		return "resync"
	case MsgStateUpdate:
		return "state_update"
	default:
		return "unknown"
	}
}

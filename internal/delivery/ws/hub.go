package ws

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/Kovalyovv/chat-app/internal/usecase"
)

type Hub struct {
	mu sync.RWMutex

	clientsByRoom map[int64]map[*Client]struct{}
	clientsByUser map[int64]map[*Client]struct{}
	roomSenders   map[int64]map[int64]struct{}
	lastDelivered map[int64]map[int64]int64 // roomID -> userID -> lastDeliveredID
	lastRead      map[int64]map[int64]int64 // roomID -> userID -> lastReadID

	Events    chan Event
	messageUC *usecase.MessageUseCase
}

type Event struct {
	Type        EventType
	RoomID      int64
	UserID      int64
	Text        string
	ClientMsgID string
	Client      *Client
	ReadUpToID  int64
	LastRecvID  int64
}

func NewHub(messageUC *usecase.MessageUseCase) *Hub {
	return &Hub{
		clientsByRoom: make(map[int64]map[*Client]struct{}),
		clientsByUser: make(map[int64]map[*Client]struct{}),
		roomSenders:   make(map[int64]map[int64]struct{}),
		lastDelivered: make(map[int64]map[int64]int64),
		lastRead:      make(map[int64]map[int64]int64),
		Events:        make(chan Event, 1024),
		messageUC:     messageUC,
	}
}

func (h *Hub) Register(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.clientsByRoom[c.RoomID] == nil {
		h.clientsByRoom[c.RoomID] = make(map[*Client]struct{})
	}
	h.clientsByRoom[c.RoomID][c] = struct{}{}

	if h.clientsByUser[c.UserID] == nil {
		h.clientsByUser[c.UserID] = make(map[*Client]struct{})
	}
	h.clientsByUser[c.UserID][c] = struct{}{}
}

func (h *Hub) Unregister(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if roomClients, ok := h.clientsByRoom[c.RoomID]; ok {
		delete(roomClients, c)
		if len(roomClients) == 0 {
			delete(h.clientsByRoom, c.RoomID)
		}
	}

	if userClients, ok := h.clientsByUser[c.UserID]; ok {
		delete(userClients, c)
		if len(userClients) == 0 {
			delete(h.clientsByUser, c.UserID)
		}
	}
}

func (h *Hub) Run() {
	for evt := range h.Events {
		switch evt.Type {

		case EventMessage:
			msgID, err := h.messageUC.Save(
				context.Background(),
				evt.RoomID,
				evt.UserID,
				evt.Text,
			)
			if err != nil {
				log.Println("save message error:", err)
				continue
			}

			h.mu.Lock()
			if h.roomSenders[evt.RoomID] == nil {
				h.roomSenders[evt.RoomID] = make(map[int64]struct{})
			}
			h.roomSenders[evt.RoomID][evt.UserID] = struct{}{}
			h.mu.Unlock()

			h.BroadcastMessage(evt.RoomID, OutgoingMessage{
				Type:      MsgMessage.String(),
				MessageID: msgID,
				UserID:    evt.UserID,
				RoomID:    evt.RoomID,
				Payload:   evt.Text,
				Timestamp: time.Now(),
			})

			evt.Client.Send <- OutgoingMessage{
				Type:        MsgAck.String(),
				MessageID:   msgID,
				ClientMsgID: evt.ClientMsgID,
			}
		case EventRead:
			h.handleRead(evt)

		case EventJoin:
			h.BroadcastSystem(evt.RoomID, MsgJoin, evt.UserID)

		case EventLeave:
			h.BroadcastSystem(evt.RoomID, MsgLeave, evt.UserID)

		case EventResync:
			h.handleResync(evt)
		}
	}
}

func (h *Hub) BroadcastMessage(roomID int64, msg OutgoingMessage) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if clients, ok := h.clientsByRoom[roomID]; ok {
		for c := range clients {
			select {
			case c.Send <- msg:
			default:
			}
		}
	}
}

func (h *Hub) BroadcastSystem(roomID int64, t MessageType, userID int64) {
	h.broadcast(roomID, OutgoingMessage{
		Type:      t.String(),
		UserID:    userID,
		RoomID:    roomID,
		Timestamp: time.Now(),
	})
}

func (h *Hub) broadcast(roomID int64, msg OutgoingMessage) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if clients, ok := h.clientsByRoom[roomID]; ok {
		for c := range clients {
			select {
			case c.Send <- msg:
			default:
			}
		}
	}
}

func (h *Hub) handleRead(evt Event) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.lastRead[evt.RoomID] == nil {
		h.lastRead[evt.RoomID] = make(map[int64]int64)
	}

	prev := h.lastRead[evt.RoomID][evt.UserID]
	if evt.ReadUpToID <= prev {
		return
	}

	h.lastRead[evt.RoomID][evt.UserID] = evt.ReadUpToID

	if err := h.messageUC.UpdateReadState(
		context.Background(),
		evt.RoomID,
		evt.UserID,
		evt.ReadUpToID,
	); err != nil {
		log.Println("update read state error:", err)
	}

	h.broadcastRead(evt.RoomID, evt.UserID, evt.ReadUpToID)
}

func (h *Hub) handleResync(evt Event) {
	if evt.ReadUpToID > 0 {
		h.handleRead(Event{
			Type:       EventRead,
			RoomID:     evt.RoomID,
			UserID:     evt.UserID,
			ReadUpToID: evt.ReadUpToID,
		})
	}

	startAfter := evt.LastRecvID
	msgs, err := h.messageUC.GetAfter(context.Background(), evt.RoomID, startAfter, 100)
	if err != nil {
		return
	}

	for _, m := range msgs {
		evt.Client.Send <- OutgoingMessage{
			Type:      MsgMessage.String(),
			MessageID: m.ID,
			UserID:    m.UserID,
			RoomID:    m.RoomID,
			Payload:   m.Text,
			Timestamp: m.CreatedAt,
		}

		go h.MarkDelivered(evt.RoomID, evt.UserID, m.ID)
	}
}

func (h *Hub) broadcastRead(roomID, readerID int64, upTo int64) {
	h.mu.RLock()
	senders := h.roomSenders[roomID]
	h.mu.RUnlock()

	for senderID := range senders {
		if senderID == readerID {
			continue
		}

		h.sendToUser(senderID, OutgoingMessage{
			Type:      MsgRead.String(),
			UserID:    readerID,
			RoomID:    roomID,
			MessageID: upTo,
			Timestamp: time.Now(),
		})
	}
}

func (h *Hub) sendToUser(userID int64, msg OutgoingMessage) {
	if clients, ok := h.clientsByUser[userID]; ok {
		for c := range clients {
			c.Send <- msg
		}
	}
}

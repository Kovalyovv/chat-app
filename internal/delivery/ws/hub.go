package ws

import (
	"context"
	"fmt"
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
	log.Println("Hub.Run started")
	for evt := range h.Events {
		switch evt.Type {
		case EventMessage:
			h.handleMessage(evt)
		case EventRead:
			h.handleRead(evt)
		case EventResync:
			h.handleResync(evt)
		}
	}
}

func (h *Hub) handleRead(evt Event) {
	ctx := context.Background()

	h.mu.Lock()

	if h.lastRead[evt.RoomID] == nil {
		h.lastRead[evt.RoomID] = make(map[int64]int64)
	}

	prev := h.lastRead[evt.RoomID][evt.UserID]
	if evt.ReadUpToID <= prev {
		h.mu.Unlock()
		log.Println("handleRead skipped, upTo <= prev")
		return
	}

	h.lastRead[evt.RoomID][evt.UserID] = evt.ReadUpToID

	senders := snapshotSenders(h.roomSenders[evt.RoomID])
	h.mu.Unlock()

	_ = h.messageUC.UpdateReadState(ctx, evt.RoomID, evt.UserID, evt.ReadUpToID)

	for senderID := range senders {
		if senderID == evt.UserID {
			continue
		}
		h.sendToUser(senderID, OutgoingMessage{
			Type:      MsgRead,
			UserID:    evt.UserID,
			RoomID:    evt.RoomID,
			MessageID: evt.ReadUpToID,
			Timestamp: time.Now().Unix(),
		})
	}
}

func (h *Hub) handleMessage(evt Event) {
	ctx := context.Background()

	msgID, err := h.messageUC.Save(ctx, evt.RoomID, evt.UserID, evt.Text)
	if err != nil {
		log.Println("save message error:", err)
		return
	}

	h.mu.Lock()
	if h.roomSenders[evt.RoomID] == nil {
		h.roomSenders[evt.RoomID] = make(map[int64]struct{})
	}
	h.roomSenders[evt.RoomID][evt.UserID] = struct{}{}

	clients := snapshotClients(h.clientsByRoom[evt.RoomID])
	h.mu.Unlock()

	out := OutgoingMessage{
		Type:      MsgMessage,
		MessageID: msgID,
		UserID:    evt.UserID,
		RoomID:    evt.RoomID,
		Payload:   evt.Text,
		Timestamp: time.Now().Unix(),
	}

	for _, c := range clients {
		select {
		case c.Send <- out:
			if c.UserID != evt.UserID {
				_ = h.messageUC.UpdateDeliveryState(
					ctx, evt.RoomID, c.UserID, msgID,
				)
			}
		default:
			log.Printf("send buffer full for user %d", c.UserID)
		}
	}

	trySend(evt.Client, OutgoingMessage{
		Type:        MsgAck,
		MessageID:   msgID,
		ClientMsgID: evt.ClientMsgID,
	})
}

func (h *Hub) handleResync(evt Event) {
	ctx := context.Background()

	state, err := h.messageUC.GetChatState(ctx, evt.RoomID, evt.UserID)
	if err != nil {
		log.Println("get state error:", err)
		return
	}

	startAfter := evt.LastRecvID
	msgs, err := h.messageUC.GetAfter(ctx, evt.RoomID, startAfter, 100)
	if err != nil {
		log.Println("get after error:", err)
		return
	}

	for _, m := range msgs {
		trySend(evt.Client, OutgoingMessage{
			Type:      MsgMessage,
			MessageID: m.ID,
			UserID:    m.UserID,
			RoomID:    m.RoomID,
			Payload:   m.Text,
			Timestamp: m.CreatedAt.Unix(),
		})

		_ = h.messageUC.UpdateDeliveryState(ctx, evt.RoomID, evt.UserID, m.ID)
	}

	if state.LastReadMessageID > evt.ReadUpToID {
		trySend(evt.Client, OutgoingMessage{
			Type:      MsgStateUpdate,
			RoomID:    evt.RoomID,
			Payload:   fmt.Sprintf("last_read: %d", state.LastReadMessageID),
			Timestamp: time.Now().Unix(),
		})
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
			Type:      MsgRead,
			UserID:    readerID,
			RoomID:    roomID,
			MessageID: upTo,
			Timestamp: time.Now().Unix(),
		})
	}
}

func trySend(c *Client, msg OutgoingMessage) {
	select {
	case c.Send <- msg:
	default:
	}
}

func snapshotClients(src map[*Client]struct{}) []*Client {
	res := make([]*Client, 0, len(src))
	for c := range src {
		res = append(res, c)
	}
	return res
}

func snapshotSenders(src map[int64]struct{}) map[int64]struct{} {
	res := make(map[int64]struct{}, len(src))
	for k := range src {
		res[k] = struct{}{}
	}
	return res
}

func (h *Hub) sendToUser(userID int64, msg OutgoingMessage) {
	h.mu.RLock()
	clients := snapshotClients(h.clientsByUser[userID])
	h.mu.RUnlock()

	for _, c := range clients {
		trySend(c, msg)
	}
}

func max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

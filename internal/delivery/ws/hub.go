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

	roomSenders map[int64]map[int64]struct{}

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

func (h *Hub) Run(ctx context.Context) {
	log.Println("Hub.Run started")
	for {
		select {
		case <-ctx.Done():
			log.Println("Hub stopping...")
			return
		case evt := <-h.Events:
			switch evt.Type {
			case EventMessage:
				go h.handleMessage(evt)
			case EventRead:
				go h.handleRead(evt)
			case EventResync:
				go h.handleResync(evt)
			}
		}
	}
}

func (h *Hub) handleMessage(evt Event) {
	dbCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	msgID, err := h.messageUC.Save(dbCtx, evt.RoomID, evt.UserID, evt.Text)
	if err != nil {
		log.Printf("[ERROR] failed to save message: %v", err)
		return
	}

	out := OutgoingMessage{
		Type:      MsgMessage,
		MessageID: msgID,
		UserID:    evt.UserID,
		RoomID:    evt.RoomID,
		Payload:   evt.Text,
		Timestamp: time.Now().Unix(),
	}

	h.mu.RLock()
	clients := snapshotClients(h.clientsByRoom[evt.RoomID])

	h.mu.RUnlock()
	h.mu.Lock()
	if h.roomSenders[evt.RoomID] == nil {
		h.roomSenders[evt.RoomID] = make(map[int64]struct{})
	}
	h.roomSenders[evt.RoomID][evt.UserID] = struct{}{}
	h.mu.Unlock()

	for _, c := range clients {
		select {
		case c.Send <- out:
			if c.UserID != evt.UserID {
				go func(uid int64) {
					_ = h.messageUC.UpdateDeliveryState(context.Background(), evt.RoomID, uid, msgID)
				}(c.UserID)
			}
		default:
			log.Printf("Send buffer full for user %d", c.UserID)
		}
	}

	trySend(evt.Client, OutgoingMessage{
		Type:        MsgAck,
		MessageID:   msgID,
		ClientMsgID: evt.ClientMsgID,
	})
}

func (h *Hub) handleRead(evt Event) {
	dbCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := h.messageUC.UpdateReadState(dbCtx, evt.RoomID, evt.UserID, evt.ReadUpToID)
	if err != nil {
		return
	}

	h.mu.RLock()
	senders := snapshotSenders(h.roomSenders[evt.RoomID])
	h.mu.RUnlock()

	msg := OutgoingMessage{
		Type:      MsgRead,
		UserID:    evt.UserID,
		RoomID:    evt.RoomID,
		MessageID: evt.ReadUpToID,
		Timestamp: time.Now().Unix(),
	}

	for senderID := range senders {
		if senderID != evt.UserID {
			h.sendToUser(senderID, msg)
		}
	}
}

func (h *Hub) handleResync(evt Event) {
	dbCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	state, _ := h.messageUC.GetChatState(dbCtx, evt.RoomID, evt.UserID)
	msgs, _ := h.messageUC.GetAfter(dbCtx, evt.RoomID, evt.LastRecvID, 100)

	for _, m := range msgs {
		trySend(evt.Client, OutgoingMessage{
			Type:      MsgMessage,
			MessageID: m.ID,
			UserID:    m.UserID,
			RoomID:    m.RoomID,
			Payload:   m.Text,
			Timestamp: m.CreatedAt.Unix(),
		})
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

func (h *Hub) sendToUser(userID int64, msg OutgoingMessage) {
	h.mu.RLock()
	clients := snapshotClients(h.clientsByUser[userID])
	h.mu.RUnlock()

	for _, c := range clients {
		trySend(c, msg)
	}
}

func trySend(c *Client, msg OutgoingMessage) {
	if c == nil {
		return
	}
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

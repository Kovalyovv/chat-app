package ws

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/Kovalyovv/chat-app/internal/usecase"
)

type Hub struct {
	mu            sync.RWMutex
	clientsByRoom map[int64]map[*Client]struct{}
	clientsByUser map[int64]map[*Client]struct{}
	roomSenders   map[int64]map[int64]struct{}
	Events        chan Event
	messageUC     *usecase.MessageUseCase
	log           *slog.Logger
}

func NewHub(messageUC *usecase.MessageUseCase, logger *slog.Logger) *Hub {
	return &Hub{
		clientsByRoom: make(map[int64]map[*Client]struct{}),
		clientsByUser: make(map[int64]map[*Client]struct{}),
		roomSenders:   make(map[int64]map[int64]struct{}),
		Events:        make(chan Event, 1024),
		messageUC:     messageUC,
		log:           logger,
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
	h.log.Info("hub started")
	for {
		select {
		case <-ctx.Done():
			h.log.Info("hub stopping")
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
	msgID, err := h.messageUC.Save(evt.Context, evt.RoomID, evt.UserID, evt.Text)
	if err != nil {
		h.log.Error("failed to save message", "error", err)
		return
	}

	out := OutgoingMessage{
		Type:        MsgMessage,
		MessageID:   msgID,
		UserID:      evt.UserID,
		RoomID:      evt.RoomID,
		Payload:     evt.Text,
		ClientMsgID: evt.ClientMsgID,
		Timestamp:   time.Now().Unix(),
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
					ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
					defer cancel()
					if err := h.messageUC.UpdateDeliveryState(ctx, evt.RoomID, uid, msgID); err != nil {
						slog.Error(
							"failed to update delivery state",
							"error", err,
							"user_id", uid,
							"room_id", evt.RoomID,
						)
					}
				}(c.UserID)
			}
		default:
			h.log.Warn("send buffer full for user", "user_id", c.UserID)
		}
	}

	trySend(evt.Client, OutgoingMessage{
		Type:        MsgAck,
		MessageID:   msgID,
		ClientMsgID: evt.ClientMsgID,
	})
}

func (h *Hub) handleRead(evt Event) {
	err := h.messageUC.UpdateReadState(evt.Context, evt.RoomID, evt.UserID, evt.ReadUpToID)
	if err != nil {
		h.log.Error("failed to update read state", "error", err)
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
	state, err := h.messageUC.GetChatState(evt.Context, evt.RoomID, evt.UserID)
	if err != nil {
		h.log.Error("resync: failed to get chat state", "error", err)
	}

	msgs, err := h.messageUC.GetAfter(evt.Context, evt.RoomID, evt.LastRecvID, 100)
	if err != nil {
		h.log.Error("resync: failed to get messages after", "error", err)
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

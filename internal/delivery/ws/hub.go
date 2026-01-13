package ws

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/Kovalyovv/chat-app/internal/domain"
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

func (h *Hub) BroadcastSystemMessage(roomID int64, uploaderID int64, objectKey string) {
	h.mu.RLock()
	clients := snapshotClients(h.clientsByRoom[roomID])
	h.mu.RUnlock()

	if len(clients) == 0 {
		h.log.Info("no active clients in room to broadcast system message", "room_id", roomID)
		return
	}

	msg := &domain.Message{
		RoomID: roomID,
		UserID: uploaderID,
		Type:   "FILE",
		Metadata: map[string]interface{}{
			"object_key": objectKey,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	msgID, err := h.messageUC.Save(ctx, msg)
	if err != nil {
		h.log.Error("failed to save system message", "error", err, "type", msg.Type, "uploader_id", uploaderID, "room_id", roomID)
		return
	}
	msg.ID = msgID

	h.broadcastMessage(msg, "")
}

func (h *Hub) broadcastMessage(msg *domain.Message, clientMsgID string) {
	out := OutgoingMessage{
		Type:        MsgMessage,
		MessageID:   msg.ID,
		UserID:      msg.UserID,
		RoomID:      msg.RoomID,
		Payload:     msg,
		ClientMsgID: clientMsgID,
		Timestamp:   time.Now().Unix(),
	}

	h.mu.RLock()
	clients := snapshotClients(h.clientsByRoom[msg.RoomID])
	h.mu.RUnlock()

	if msg.UserID != 0 {
		h.mu.Lock()
		if h.roomSenders[msg.RoomID] == nil {
			h.roomSenders[msg.RoomID] = make(map[int64]struct{})
		}
		h.roomSenders[msg.RoomID][msg.UserID] = struct{}{}
		h.mu.Unlock()
	}

	for _, c := range clients {
		select {
		case c.Send <- out:
			if c.UserID != msg.UserID {
				go func(recipientID int64) {
					ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
					defer cancel()
					if err := h.messageUC.UpdateDeliveryState(ctx, msg.RoomID, recipientID, msg.ID); err != nil {
						h.log.Error("failed to update delivery state", "error", err, "recipient_id", recipientID,
							"room_id", msg.RoomID,
						)
					}
				}(c.UserID)
			}
		default:
			h.log.Warn("send buffer full for user", "user_id", c.UserID)
		}
	}
}

func (h *Hub) handleMessage(evt Event) {
	msg := &domain.Message{
		RoomID: evt.RoomID,
		UserID: evt.UserID,
		Type:   "TEXT",
		Text:   evt.Text,
	}

	msgID, err := h.messageUC.Save(evt.Context, msg)
	if err != nil {
		h.log.Error("failed to save text message", "error", err, "user_id",
			msg.UserID, "room_id", msg.RoomID)
		return
	}
	msg.ID = msgID

	h.broadcastMessage(msg, evt.ClientMsgID)

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
		return
	}

	for _, m := range msgs {
		trySend(evt.Client, OutgoingMessage{
			Type:      MsgMessage,
			MessageID: m.ID,
			UserID:    m.UserID,
			RoomID:    m.RoomID,
			Payload:   m,
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

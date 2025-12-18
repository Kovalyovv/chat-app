package ws

import (
	"context"
	"log"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = (pongWait * 9) / 10
)

type Client struct {
	UserID int64
	RoomID int64
	Hub    *Hub
	Conn   *websocket.Conn
	Send   chan OutgoingMessage
}

func NewClient(userID, roomID int64, conn *websocket.Conn, hub *Hub) *Client {
	return &Client{
		UserID: userID,
		RoomID: roomID,
		Hub:    hub,
		Conn:   conn,
		Send:   make(chan OutgoingMessage, 256),
	}
}

func (c *Client) ReadPump() {
	defer func() {
		c.Hub.Unregister(c)
		c.Hub.BroadcastSystem(c.RoomID, MsgLeave, c.UserID)
		c.Conn.Close()
	}()

	c.Conn.SetReadLimit(512)
	c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		var msg IncomingMessage
		if err := c.Conn.ReadJSON(&msg); err != nil {
			if websocket.IsUnexpectedCloseError(
				err,
				websocket.CloseGoingAway,
				websocket.CloseAbnormalClosure,
			) {
				log.Println("read error:", err)
			}
			break
		}

		switch msg.Type {
		case MsgMessage:
			text, ok := msg.Payload["text"].(string)
			if !ok || text == "" {
				continue
			}

			clientMsgID, _ := msg.Payload["client_msg_id"].(string)

			c.Hub.Events <- Event{
				Type:        EventMessage,
				RoomID:      c.RoomID,
				UserID:      c.UserID,
				Text:        text,
				ClientMsgID: clientMsgID,
				Client:      c,
			}

		case MsgRead:
			upTo, ok := msg.Payload["up_to_message_id"].(float64)
			if !ok || upTo <= 0 {
				continue
			}

			c.Hub.Events <- Event{
				Type:       EventRead,
				RoomID:     c.RoomID,
				UserID:     c.UserID,
				ReadUpToID: int64(upTo),
			}

		case MsgResync:
			lastRecv, _ := msg.Payload["last_received_message_id"].(float64)
			lastRead, _ := msg.Payload["last_read_message_id"].(float64)

			c.Hub.Events <- Event{
				Type:       EventResync,
				RoomID:     c.RoomID,
				UserID:     c.UserID,
				ReadUpToID: int64(lastRead),
				Client:     c,
				LastRecvID: int64(lastRecv),
			}

		default:
		}
	}
}

func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.Conn.WriteJSON(msg); err != nil {
				return
			}

			if msg.Type == MsgMessage.String() && msg.MessageID != 0 {
				go c.Hub.MarkDelivered(c.RoomID, c.UserID, msg.MessageID)
			}

		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (h *Hub) MarkDelivered(roomID, userID, messageID int64) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.lastDelivered[roomID] == nil {
		h.lastDelivered[roomID] = make(map[int64]int64)
	}

	prev := h.lastDelivered[roomID][userID]
	if messageID <= prev {
		return
	}

	h.lastDelivered[roomID][userID] = messageID

	go func() {
		if err := h.messageUC.UpdateDeliveryState(context.Background(), roomID, userID, messageID); err != nil {
			log.Println("failed to save delivery state:", err)
		}
	}()

	h.NotifyDeliveredToSender(roomID, userID, messageID)
}

func (h *Hub) NotifyDeliveredToSender(roomID, receiverID int64, messageID int64) {
	h.mu.RLock()
	senders := h.roomSenders[roomID]
	h.mu.RUnlock()

	for senderID := range senders {
		if senderID == receiverID {
			continue
		}
		h.sendToUser(senderID, OutgoingMessage{
			Type:      MsgDelivered.String(),
			UserID:    receiverID,
			RoomID:    roomID,
			MessageID: messageID,
			Timestamp: time.Now(),
		})
	}
}

func (h *Hub) NotifyDelivered(userID int64, messageID int64) {
	h.mu.RLock()
	clients := h.clientsByUser[userID]
	h.mu.RUnlock()

	for c := range clients {
		c.Send <- OutgoingMessage{
			Type:      MsgDelivered.String(),
			MessageID: messageID,
			Timestamp: time.Now(),
		}
	}
}

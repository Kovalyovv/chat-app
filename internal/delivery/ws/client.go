package ws

import (
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
	UserID int
	RoomID int
	Hub    *Hub
	Conn   *websocket.Conn
	Send   chan OutgoingMessage
}

func NewClient(userID, roomID int, conn *websocket.Conn, hub *Hub) *Client {
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
				c.Hub.MarkDelivered(c.UserID, c.RoomID, msg.MessageID)
			}

		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (h *Hub) MarkDelivered(userID, roomID int, messageID int64) {
	h.NotifyDelivered(userID, messageID)
}

func (h *Hub) NotifyDelivered(userID int, messageID int64) {
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

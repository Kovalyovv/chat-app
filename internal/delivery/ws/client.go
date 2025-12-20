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
	maxMsgSize = 4 << 10 // 4KB
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
		Send:   make(chan OutgoingMessage, 1024),
	}
}

// ===================== read =====================

func (c *Client) ReadPump() {
	defer func() {
		c.Hub.Unregister(c)
		_ = c.Conn.Close()
	}()

	c.Conn.SetReadLimit(maxMsgSize)
	_ = c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetPongHandler(func(string) error {
		_ = c.Conn.SetReadDeadline(time.Now().Add(pongWait))
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
				log.Println("ws read error:", err)
			}
			return
		}

		switch msg.Type {

		case MsgSend:
			p, err := DecodePayload[SendPayload](msg)
			if err != nil {
				log.Println("decode send payload:", err)
				continue
			}

			c.Hub.Events <- Event{
				Type:        EventMessage,
				Text:        p.Text,
				ClientMsgID: p.ClientMsgID,
				Client:      c,
				RoomID:      c.RoomID,
				UserID:      c.UserID,
			}

		case MsgRead:
			p, err := DecodePayload[ReadPayload](msg)
			if err != nil {
				log.Println("decode read payload:", err)
				continue
			}

			c.Hub.Events <- Event{
				Type:       EventRead,
				ReadUpToID: p.UpToMessageID,
				Client:     c,
				RoomID:     c.RoomID,
				UserID:     c.UserID,
			}

		case MsgResync:
			p, err := DecodePayload[ResyncPayload](msg)
			if err != nil {
				log.Println("decode resync payload:", err)
				continue
			}

			c.Hub.Events <- Event{
				Type:       EventResync,
				RoomID:     c.RoomID,
				UserID:     c.UserID,
				ReadUpToID: p.ReadUpToMessageID,
				LastRecvID: p.LastRecvMessageID,
				Client:     c,
			}

		default:
			log.Println("unknown ws message type:", msg.Type)
		}
	}
}

// ===================== write =====================

func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.Conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.Send:
			_ = c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.Conn.WriteJSON(msg); err != nil {
				return
			}

		case <-ticker.C:
			_ = c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

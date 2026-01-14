package ws

import (
	"context"
	"log/slog"
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
	log    *slog.Logger
}

func NewClient(userID, roomID int64, conn *websocket.Conn, hub *Hub, logger *slog.Logger) *Client {
	return &Client{
		UserID: userID,
		RoomID: roomID,
		Hub:    hub,
		Conn:   conn,
		Send:   make(chan OutgoingMessage, 1024),
		log:    logger,
	}
}

func (c *Client) ReadPump(ctx context.Context) {
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
				c.log.Warn("ws read error", "error", err)
			}
			return
		}
		event := Event{
			Client:  c,
			RoomID:  c.RoomID,
			UserID:  c.UserID,
			Context: ctx,
		}

		switch msg.Type {

		case MsgSend:
			p, err := DecodePayload[SendPayload](msg)
			if err != nil {
				c.log.Warn("decode send payload failed", "error", err)
				continue
			}
			event.Type = EventMessage
			event.Text = p.Text
			event.ClientMsgID = p.ClientMsgID

		case MsgRead:
			p, err := DecodePayload[ReadPayload](msg)
			if err != nil {
				c.log.Warn("decode read payload failed", "error", err)
				continue
			}
			event.Type = EventRead
			event.ReadUpToID = p.UpToMessageID

		case MsgResync:
			p, err := DecodePayload[ResyncPayload](msg)
			if err != nil {
				c.log.Warn("decode resync payload failed", "error", err)
				continue
			}
			event.Type = EventResync
			event.ReadUpToID = p.ReadUpToMessageID
			event.LastRecvID = p.LastRecvMessageID

		default:
			c.log.Warn("unknown ws message type", "type", msg.Type)
			continue
		}
		c.Hub.Events <- event
	}
}

func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.Conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.Send:
			if err := c.Conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				c.log.Warn("failed to set write deadline", "error", err)
			}
			if !ok {
				if err := c.Conn.WriteMessage(websocket.CloseMessage, []byte{}); err != nil {
					c.log.Warn("failed to send close message", "error", err)
				}
				return
			}

			if err := c.Conn.WriteJSON(msg); err != nil {
				c.log.Warn("ws write error", "error", err)
				return
			}

		case <-ticker.C:
			_ = c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				c.log.Warn("ws ping error", "error", err)
				return
			}
		}
	}
}

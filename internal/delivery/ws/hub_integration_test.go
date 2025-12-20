package ws_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/Kovalyovv/chat-app/internal/delivery/ws"
	"github.com/Kovalyovv/chat-app/internal/repository/postgres"
	"github.com/Kovalyovv/chat-app/internal/usecase"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	psqlTest "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

const testDDL = `
CREATE TABLE IF NOT EXISTS rooms (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    invite_code VARCHAR(16) UNIQUE NOT NULL,
    owner_id INT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS room_members (
    user_id INT NOT NULL,
    room_id INT NOT NULL,
    PRIMARY KEY (user_id, room_id)
);

CREATE TABLE IF NOT EXISTS messages (
    id SERIAL PRIMARY KEY,
    room_id INT NOT NULL,
    user_id INT NOT NULL,
    text TEXT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE IF NOT EXISTS room_read_states (
    room_id INT NOT NULL,
    user_id INT NOT NULL,
    last_read_message_id BIGINT NOT NULL DEFAULT 0,
    last_delivered_message_id BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (room_id, user_id),
    FOREIGN KEY (room_id) REFERENCES rooms(id) ON DELETE CASCADE
);

DO $$
BEGIN
    ALTER TABLE room_members
        ADD FOREIGN KEY (room_id) REFERENCES rooms (id) ON DELETE CASCADE;
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE messages
        ADD FOREIGN KEY (room_id) REFERENCES rooms (id) ON DELETE CASCADE;
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;

CREATE INDEX IF NOT EXISTS idx_messages_room_created
    ON messages (room_id, created_at DESC);
`

func TestHub_FullFlow_Delivery_Read_Resync(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	store := &postgres.Store{
		Pool:          pool,
		RoomRepo:      postgres.NewRoomRepo(pool),
		MessageRepo:   postgres.NewMessageRepo(pool),
		ChatStateRepo: postgres.NewChatStateRepo(pool),
	}

	roomUC := usecase.NewRoomUseCase(store)
	messageUC := usecase.NewMessageUseCase(store)

	hub := ws.NewHub(messageUC)
	go hub.Run()

	gin.SetMode(gin.TestMode)
	router := gin.New()

	authMW := func(c *gin.Context) {
		if uid := c.Query("X-User-ID"); uid != "" {
			id, _ := strconv.ParseInt(uid, 10, 64)
			c.Set("userID", id)
		}
		c.Next()
	}

	wsHandler := ws.NewWSHandler(hub, roomUC, messageUC)
	api := router.Group("/api/v1")
	api.GET("/ws/:roomId", authMW, wsHandler.Handle)

	ts := httptest.NewServer(router)
	defer ts.Close()

	ctx := context.Background()
	room, err := roomUC.CreateRoom(ctx, "test-room", 1)
	require.NoError(t, err)

	require.NoError(t, store.RoomRepo.AddMember(ctx, 1, room.ID))
	require.NoError(t, store.RoomRepo.AddMember(ctx, 2, room.ID))

	wsURL := "ws" + ts.URL[4:] + fmt.Sprintf("/api/v1/ws/%d", room.ID)

	// Подключение клиентов
	conn1, _, err := websocket.DefaultDialer.Dial(wsURL+"?X-User-ID=1", nil)
	require.NoError(t, err)
	t.Cleanup(func() { conn1.Close() })

	conn2, _, err := websocket.DefaultDialer.Dial(wsURL+"?X-User-ID=2", nil)
	require.NoError(t, err)
	t.Cleanup(func() { conn2.Close() })

	time.Sleep(200 * time.Millisecond) // небольшая пауза после подключения

	sendMessage(t, conn1, "привет", "msg-1")

	msg := readExpectedMessage(t, conn2, ws.MsgMessage)
	assert.Equal(t, "привет", msg.Payload)

	time.Sleep(100 * time.Millisecond)

	state, err := messageUC.GetChatState(ctx, room.ID, 2)
	require.NoError(t, err)
	assert.Equal(t, msg.MessageID, state.LastDeliveredMessageID)

	sendRead(t, conn2, msg.MessageID)

	readMsg := readExpectedMessage(t, conn1, ws.MsgRead)
	assert.Equal(t, int64(2), readMsg.UserID)

	conn2.Close()
	conn2, _, err = websocket.DefaultDialer.Dial(wsURL+"?X-User-ID=2", nil)
	require.NoError(t, err)
	t.Cleanup(func() { conn2.Close() })

	time.Sleep(200 * time.Millisecond)

	historyMsg := readExpectedMessage(t, conn2, ws.MsgHistory)
	assert.Equal(t, "привет", historyMsg.Payload)
	assert.Equal(t, int64(1), historyMsg.UserID)

	_ = conn2.SetReadDeadline(time.Time{})

	sendResync(t, conn2, 0, 0)

	resyncMsg := readExpectedMessage(t, conn2, ws.MsgMessage)
	assert.Equal(t, msg.MessageID, resyncMsg.MessageID)

	stateUpdate := readExpectedMessage(t, conn2, ws.MsgStateUpdate)
	assert.Contains(t, stateUpdate.Payload.(string), fmt.Sprintf("last_read: %d", msg.MessageID))
}

func sendMessage(t *testing.T, conn *websocket.Conn, text, clientMsgID string) {
	payload := ws.SendPayload{Text: text, ClientMsgID: clientMsgID}
	data, _ := json.Marshal(payload)
	require.NoError(t, conn.WriteJSON(ws.IncomingMessage{Type: ws.MsgSend, Payload: data}))
}

func sendRead(t *testing.T, conn *websocket.Conn, upTo int64) {
	payload := ws.ReadPayload{UpToMessageID: upTo}
	data, _ := json.Marshal(payload)
	require.NoError(t, conn.WriteJSON(ws.IncomingMessage{Type: ws.MsgRead, Payload: data}))
}

func sendResync(t *testing.T, conn *websocket.Conn, lastRecv, lastRead int64) {
	payload := ws.ResyncPayload{LastRecvMessageID: lastRecv, ReadUpToMessageID: lastRead}
	data, _ := json.Marshal(payload)
	require.NoError(t, conn.WriteJSON(ws.IncomingMessage{Type: ws.MsgResync, Payload: data}))
}

func readExpectedMessage(t *testing.T, conn *websocket.Conn, expected ws.MessageType) ws.OutgoingMessage {
	t.Helper()

	require.NoError(t, conn.SetReadDeadline(time.Now().Add(10*time.Second)))

	for {
		var msg ws.OutgoingMessage
		if err := conn.ReadJSON(&msg); err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				t.Fatalf("timeout waiting for message type %s", expected)
			}
			t.Fatalf("read error: %v", err)
		}

		if msg.Type == expected {
			return msg
		}
	}
}

func setupTestDB(t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()

	ctx := context.Background()

	pgContainer, err := psqlTest.Run(ctx,
		"postgres:16-alpine",
		psqlTest.WithDatabase("testdb"),
		psqlTest.WithUsername("test"),
		psqlTest.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	require.NoError(t, err)

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, testDDL)
	require.NoError(t, err)

	cleanup := func() {
		pool.Close()
		_ = pgContainer.Terminate(ctx)
	}

	return pool, cleanup
}

//func readExpectedMessage(t *testing.T, conn *websocket.Conn, expected ws.MessageType) ws.OutgoingMessage {
//	t.Helper()
//
//	if err := conn.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
//		t.Fatalf("set read deadline failed: %v", err)
//	}
//
//	for {
//		var msg ws.OutgoingMessage
//		err := conn.ReadJSON(&msg)
//		if err != nil {
//			if ne, ok := err.(net.Error); ok && ne.Timeout() {
//				t.Fatalf("timeout waiting for %s", expected)
//			}
//			t.Fatalf("websocket read failed: %v", err)
//		}
//
//		if msg.Type == expected {
//			return msg
//		}
//	}
//}

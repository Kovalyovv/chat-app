package ws_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Kovalyovv/chat-app/internal/delivery/ws"
	"github.com/Kovalyovv/chat-app/internal/domain"
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
    message_type VARCHAR(20) NOT NULL,
    text TEXT,
    metadata JSONB,
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
`

type TestMessage struct {
	Type      ws.MessageType `json:"type"`
	MessageID int64          `json:"message_id"`
	UserID    int64          `json:"user_id"`
	Payload   struct {
		Text string `json:"text"`
		ID   int64  `json:"id"`
	} `json:"payload"`
}

func TestHub_FullFlow_Delivery_Read_Resync(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	roomRepo := postgres.NewRoomRepo(pool)
	messageRepo := postgres.NewMessageRepo(pool)
	chatStateRepo := postgres.NewChatStateRepo(pool)

	roomUC := usecase.NewRoomUseCase(roomRepo)
	messageUC := usecase.NewMessageUseCase(messageRepo, chatStateRepo)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	systemMessages := make(chan *domain.Message, 1)
	hub := ws.NewHub(messageUC, systemMessages, logger)
	hubCtx, cancelHub := context.WithCancel(context.Background())
	defer cancelHub()
	go hub.Run(hubCtx)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	authMW := func(c *gin.Context) {
		id, _ := strconv.ParseInt(c.Query("X-User-ID"), 10, 64)
		c.Set("userID", id)
		c.Next()
	}
	wsHandler := ws.NewWSHandler(hub, roomUC, messageUC, 50, logger)
	router.GET("/ws/:roomId", authMW, wsHandler.Handle)
	ts := httptest.NewServer(router)
	defer ts.Close()

	ctx := context.Background()
	room, err := roomUC.CreateRoom(ctx, "test-room", 1)
	require.NoError(t, err)
	require.NoError(t, roomRepo.AddMember(ctx, 2, room.ID))

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + fmt.Sprintf("/ws/%d", room.ID)
	conn1 := connectOrFail(t, wsURL, 1)
	conn2 := connectOrFail(t, wsURL, 2)

	sendMessage(t, conn1, "Hello", "client-msg-1")

	receivedMsg := readExpectedMessage(t, conn2, ws.MsgMessage)
	assert.Equal(t, "Hello", receivedMsg.Payload.Text, "Payload text should match")
	assert.NotZero(t, receivedMsg.MessageID, "Message ID should be set by the server")

	firstMessageID := receivedMsg.MessageID

	time.Sleep(200 * time.Millisecond) // Allow time for the async DB update
	state, err := messageUC.GetChatState(ctx, room.ID, 2)
	require.NoError(t, err)
	assert.Equal(t, firstMessageID, state.LastDeliveredMessageID)

	sendRead(t, conn2, firstMessageID)

	readReceipt := readExpectedMessage(t, conn1, ws.MsgRead)
	assert.Equal(t, firstMessageID, readReceipt.MessageID)
	assert.Equal(t, int64(2), readReceipt.UserID, "Read receipt should be from User 2")

	conn2.Close()
	conn2 = connectOrFail(t, wsURL, 2)

	historyMsg := readExpectedMessage(t, conn2, ws.MsgHistory)
	assert.Equal(t, "Hello", historyMsg.Payload.Text, "History message payload should match")
	assert.Equal(t, firstMessageID, historyMsg.Payload.ID, "History message ID should match")
}

func connectOrFail(t *testing.T, url string, userID int64) *websocket.Conn {
	t.Helper()
	conn, _, err := websocket.DefaultDialer.Dial(url+fmt.Sprintf("?X-User-ID=%d", userID), nil)
	require.NoError(t, err, "Failed to connect for user", userID)
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func sendMessage(t *testing.T, conn *websocket.Conn, text, clientMsgID string) {
	t.Helper()
	payload := ws.SendPayload{Text: text, ClientMsgID: clientMsgID}
	data, _ := json.Marshal(payload)
	require.NoError(t, conn.WriteJSON(ws.IncomingMessage{Type: ws.MsgSend, Payload: data}))
}

func sendRead(t *testing.T, conn *websocket.Conn, upTo int64) {
	t.Helper()
	payload := ws.ReadPayload{UpToMessageID: upTo}
	data, _ := json.Marshal(payload)
	require.NoError(t, conn.WriteJSON(ws.IncomingMessage{Type: ws.MsgRead, Payload: data}))
}

func readExpectedMessage(t *testing.T, conn *websocket.Conn, expectedType ws.MessageType) TestMessage {
	t.Helper()
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(10*time.Second)))

	for {
		_, msgBytes, err := conn.ReadMessage()
		require.NoError(t, err, "Failed to read raw message from websocket")

		var genericMsg TestMessage
		err = json.Unmarshal(msgBytes, &genericMsg)
		require.NoError(t, err, "Failed to unmarshal message into TestMessage struct")

		if genericMsg.Type == expectedType {
			return genericMsg
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

	return pool, func() {
		pool.Close()
		_ = pgContainer.Terminate(ctx)
	}
}

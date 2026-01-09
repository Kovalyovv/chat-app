# Real-Time Chat Application (chat-app)

[![Go Version](https://img.shields.io/badge/go-1.21+-blue.svg)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

This project is the backend service for a real-time, room-based chat application. It is built using Go and follows Clean Architecture principles, interacting with a separate `auth-service` for user authentication via gRPC. It features real-time messaging, delivery status, and read receipts through WebSockets.

## Demo

A brief demonstration of the simple web UI interacting with the backend services.


![Chat App Demo](example.webm)

## Core Features

-   **Microservice Architecture:** Communicates with a separate `auth-service` for robust user authentication and token validation.
-   **Room-Based Chat:**
    -   Create new chat rooms.
    -   Join existing rooms using a unique, auto-generated invite code.
    -   View a list of all rooms a user is a member of.
-   **Real-Time Messaging:**
    -   Live message delivery using WebSockets.
    -   Broadcasts messages to all connected members of a room.
-   **Message Status & History:**
    -   **Message History:** Upon connecting, a user receives the last 50 messages from the room.
    -   **Delivery Status:** Senders receive an acknowledgment (`✓`) that the server has saved their message.
    -   **Read Receipts:** Senders receive a real-time update (`✓✓`) when other users have received and read their messages.
-   **State Synchronization:** Includes a `resync` mechanism for clients to fetch any messages missed during a disconnection.

## Architecture

The service is built following **Clean Architecture** principles to ensure a clear separation of concerns, high cohesion, and low coupling.

-   **Domain:** Contains the core business models (`Room`, `Message`) and enterprise-wide business rules.
-   **Usecase:** Implements application-specific business logic, orchestrating data flow between the domain and repositories.
-   **Repository:** Defines interfaces for data persistence. The implementation (`/internal/repository/postgres`) handles all communication with the PostgreSQL database.
-   **Delivery:** Exposes the application to the outside world. This includes:
    -   **HTTP Delivery (`/internal/delivery/http`):** A Gin-based RESTful API for managing rooms.
    -   **WebSocket Delivery (`/internal/delivery/ws`):** A Gorilla WebSocket implementation for real-time communication, managed by a central `Hub`.

## Technology Stack

-   **Language:** Go (Golang)
-   **API Framework:** [Gin](https://github.com/gin-gonic/gin) for the HTTP server.
-   **Real-Time Communication:** [Gorilla WebSocket](https://github.com/gorilla/websocket) for WebSocket connections.
-   **Database:** PostgreSQL, accessed via the [pgx/v5](https://github.com/jackc/pgx) driver.
-   **Inter-Service Communication:** gRPC for making authentication calls to the `auth-service`.
-   **Logging:** [slog](https://pkg.go.dev/log/slog) (standard library) for structured, production-ready logging.
-   **Configuration:** [godotenv](https://github.com/joho/godotenv) for managing environment variables.
-   **Testing:**
    -   [testify](https://github.com/stretchr/testify) for assertions and mocking.
    -   [testcontainers-go](https://github.com/testcontainers/testcontainers-go) for reliable integration testing with a real PostgreSQL database in Docker.

## API Endpoints

All endpoints are prefixed with `/api/v1`. Authentication is required for all endpoints via a `Bearer <token>`.

### HTTP API

| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `POST` | `/rooms` | Creates a new chat room. The user making the request becomes the owner. |
| `POST` | `/rooms/join` | Joins a user to an existing room using a `room_id` and `invite_code`. |
| `GET` | `/rooms` | Retrieves a list of all rooms the authenticated user is a member of. |

### WebSocket API

#### Connection

-   `GET /ws/:roomId`
    -   Initiates a WebSocket connection to a specific chat room.
    -   Authentication is passed via a `token` query parameter: `ws://host/api/v1/ws/1?token=<access_token>`.

#### Incoming Messages (Client -> Server)

| Type | Name | Payload | Description |
| :--- | :--- | :--- | :--- |
| 1 | `MsgSend` | `{ "text": "...", "client_msg_id": "..." }` | Sends a new message to the current room. |
| 7 | `MsgRead` | `{ "up_to_message_id": 123 }` | Informs the server that the client has read all messages up to the given ID. |
| 8 | `MsgResync` | `{ "last_recv_message_id": 120 }` | Requests all messages that have been sent since the client's last known message ID. |

#### Outgoing Messages (Server -> Client)

| Type | Name | Payload | Description |
| :--- | :--- | :--- | :--- |
| 2 | `MsgMessage` | The message text. | A new message broadcast to all room members. |
| 10 | `MsgHistory` | The message text. | A single message from the room's history, sent on connection. |
| 5 | `MsgAck` | `N/A` | Acknowledges to the sender that their message was successfully saved. |
| 7 | `MsgRead` | `N/A` | Informs a sender that another user has read their message(s). |

## Getting Started

### Prerequisites

-   Go 1.21+
-   PostgreSQL
-   Docker (for running tests)
-   A running instance of the corresponding `auth-service`.

### Installation & Setup

1.  **Clone the repository:**
    ```bash
    git clone https://github.com/Kovalyovv/chat-app.git
    cd chat-app
    ```

2.  **Install dependencies:**
    ```bash
    go mod tidy
    ```

3.  **Configure environment:**
    Create a `.env` file in the root of the project and add the required variables.

    ```env
    # The port for the chat-app's HTTP and WebSocket server
    API_PORT=8080

    # Connection string for your PostgreSQL database
    DB_URL=postgres://user:password@localhost:5432/chat_db?sslmode=disable

    # Address of the running auth-service's gRPC server
    AUTH_SERVICE_ADDR=localhost:50001
    ```

### Running the Service

Execute the main application:
```bash
go run cmd/api/main.go
```
The server will start and log that it is listening on the configured port.

## Running Tests

The project includes a comprehensive integration test suite for the WebSocket hub.

To run all tests, execute the following command from the root directory:
```bash
go test -v ./...
```

The primary test, `TestHub_FullFlow_Delivery_Read_Resync`, uses `testcontainers-go` to automatically spin up a temporary PostgreSQL Docker container. It tests the complete lifecycle of a WebSocket connection, including:
-   Sending and receiving messages between two clients.
-   Verifying message delivery state is updated in the database.
-   Confirming that `MsgRead` events are correctly broadcast.
-   Testing client reconnection and `resync` logic.
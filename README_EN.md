# Real-Time Chat Service (chat-app)

[![Go Version](https://img.shields.io/badge/go-1.24+-blue.svg)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

This project is the backend for a real-time, room-based chat application. It is built using Go, follows Clean Architecture principles, and leverages WebSockets for live communication. It interacts with other services for authentication and notifications.

![Chat App Demo](example.webm)

## Project Architecture

This project consists of three core microservices that work together:

-   **[Auth Service](../auth-service/README_EN.md):** Manages user identity. This chat service uses it to authenticate users connecting via WebSocket.
-   **`chat-app` (This Service):** Provides real-time messaging, room management, and message history.
-   **[Media Processor](../media-processor/README_EN.md):** Handles file uploads. After processing a file, it sends a notification back to this chat service to inform users in a room.

## Core Features

-   **Room-Based Chat:** Create new rooms, join existing rooms with an invite code, and view a list of your rooms.
-   **Real-Time Messaging:** Live message delivery to all connected room members using WebSockets.
-   **Message Status:** Includes acknowledgments for message delivery (`✓`) and read receipts (`✓✓`).
-   **Message History & Resync:** Fetches recent message history on connection and allows clients to request missed messages after a disconnection.
-   **Internal Notification API:** An internal endpoint for other services (like `media-processor`) to post system messages into a chat room.

## Technology Stack

-   **Language:** Go
-   **API Framework:** [Gin](https://github.com/gin-gonic/gin) for the HTTP server.
-   **Real-Time Communication:** [Gorilla WebSocket](https://github.com/gorilla/websocket).
-   **Database:** PostgreSQL, accessed via the [pgx/v5](https://github.com/jackc/pgx) driver.
-   **Inter-Service Communication:** gRPC for authentication calls to the `auth-service`.
-   **Logging:** `slog` (standard library) for structured logging.
-   **Observability & Storage:**
    -   **[Prometheus](https://prometheus.io/):** Exposes a `/metrics` endpoint for collecting application and business metrics.
    -   **[Jaeger](https://www.jaegertracing.io/):** Fully integrated for distributed tracing, allowing you to monitor request flows across all microservices.
    -   **[MinIO](https://min.io/):** Used as the S3-compatible object storage for all user-uploaded media files, managed by the `media-processor` service.


## API Endpoints

All endpoints are prefixed with `/api/v1`.

### HTTP API

| Method | Endpoint             | Description                                                  |
| :----- | :------------------- | :----------------------------------------------------------- |
| `POST` | `/rooms`             | Creates a new chat room.                                     |
| `POST` | `/rooms/join`        | Joins a user to an existing room via an invite code.       |
| `GET`  | `/rooms`             | Retrieves all rooms the authenticated user is a member of. |
| `POST` | `/internal/notify`   | **Internal Use Only.** Allows other services to post notifications. |

### WebSocket API

-   `GET /ws/:roomId?token=<access_token>`
    -   Initiates a WebSocket connection to a chat room.

## How to Run

### Recommended Method: Docker Compose

The easiest and recommended way to run the entire project is with Docker Compose. This will start this service, its database, and all other related services, including Prometheus and Jaeger.

1.  Navigate to the project's root directory.
2.  Run the following command:
    ```bash
    docker compose up --build
    ```
- **Jaeger UI:** Available at `http://localhost:16686`
- **Prometheus UI:** Available at `http://localhost:9090`

### Local Development (Alternative)

1.  **Prerequisites:**
    -   Go 1.24+ installed.
    -   A running PostgreSQL instance.
    -   A running `auth-service` instance.

2.  **Configure Environment:**
    Create a `.env` file in the `chat-app` directory:
    ```env
    # Port for the HTTP and WebSocket server
    API_PORT=8080

    # Connection string for your PostgreSQL database
    DB_URL=postgres://user:password@localhost:5432/chat_db?sslmode=disable

    # Address of the running auth-service's gRPC server
    AUTH_SERVICE_ADDR=localhost:50001
    
    # Address of the Jaeger OTLP gRPC endpoint
    JAEGER_URL=localhost:4317

    # A secret key to protect the internal notification endpoint
    INTERNAL_API_KEY=your-internal-api-key
    ```

3.  **Run the service:**
    ```bash
    go run ./cmd/api/main.go
    ```

# chat-app/Dockerfile
# Stage 1: Build the Go binary
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code
COPY . .

# Build the chat service binary
RUN CGO_ENABLED=0 GOOS=linux go build -o /chat-app-api ./cmd/api/main.go

# Stage 2: Create a minimal final image
FROM alpine:latest

# Copy the CA certificates for making TLS/HTTPS calls
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy the built binary from the builder stage
COPY --from=builder /chat-app-api /chat-app-api

# Expose the HTTP and WebSocket port
EXPOSE 8080

# Command to run the service
CMD ["/chat-app-api"]

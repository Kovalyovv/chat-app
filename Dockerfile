# Dockerfile

# --- Этап 1: Сборка ---

FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

# Собираем приложение.
# CGO_ENABLED=0 - отключает CGO, чтобы получить статически скомпилированный бинарник
# -o ./out/app - бинарник будет лежать в папке /app/out/app
RUN CGO_ENABLED=0 GOOS=linux go build -o ./out/app ./cmd/api/main.go

# --- Этап 2: Финальный образ ---
# Используем минимальный образ Alpine Linux
FROM alpine:latest

WORKDIR /root/

COPY --from=builder /app/out/app .

CMD ["./app"]

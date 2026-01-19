FROM golang:1.25.5-alpine AS builder
WORKDIR /app

COPY go.mod go.sum ./

RUN go mod edit -dropreplace=github.com/Kovalyovv/auth-service

RUN go mod tidy

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -o /server/ ./cmd/...

FROM gcr.io/distroless/static-debian11

COPY --from=builder /server /server

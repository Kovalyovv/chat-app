package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/Kovalyovv/auth-service/pkg/pb"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
)

func NewAuthMiddleware(authServiceAddr string, logger *slog.Logger) (gin.HandlerFunc, *grpc.ClientConn) {
	conn, err := grpc.Dial(
		authServiceAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithConnectParams(grpc.ConnectParams{
			Backoff: backoff.Config{
				BaseDelay:  1.0 * time.Second,
				Multiplier: 1.6,
				Jitter:     0.2,
				MaxDelay:   120 * time.Second,
			},
			MinConnectTimeout: 20 * time.Second,
		}),
	)

	if err != nil {
		logger.Error("failed to create gRPC client for auth service", "error", err, "addr", authServiceAddr)
	}

	client := pb.NewAuthServiceClient(conn)

	return func(c *gin.Context) {
		var token string

		// For WebSocket, the browser cannot send an Authorization header.
		// So we check for the token in a query parameter first.
		// The "Upgrade" header indicates a WebSocket connection attempt.
		if c.GetHeader("Upgrade") == "websocket" {
			token = c.Query("token")
		}

		if token == "" {
			authHeader := c.GetHeader("Authorization")
			if authHeader == "" {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing authorization token"})
				return
			}
			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token format"})
				return
			}
			token = parts[1]
		}

		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authorization token not found"})
			return
		}

		if conn.GetState() != connectivity.Ready {
			logger.Warn("auth service is not ready", "state", conn.GetState().String())
			ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
			defer cancel()
			if !conn.WaitForStateChange(ctx, conn.GetState()) {
				c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "authentication service is unavailable"})
				return
			}
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()

		resp, err := client.VerifyToken(ctx, &pb.VerifyTokenRequest{
			Token: token,
		})

		if err != nil || !resp.GetValid() {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}

		c.Set("userID", resp.GetUserId())
		c.Next()
	}, conn
}

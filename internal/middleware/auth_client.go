package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/Kovalyovv/auth-service/pkg/pb"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func NewAuthMiddleware(authServiceAddr string) gin.HandlerFunc {
	conn, err := grpc.Dial(authServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic("could not connect to auth service: " + err.Error())
	}
	client := pb.NewAuthServiceClient(conn)

	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing authorization header"})
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token format"})
			return
		}

		resp, err := client.VerifyToken(context.Background(), &pb.VerifyTokenRequest{
			Token: parts[1],
		})

		if err != nil || !resp.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}

		c.Set("userID", resp.UserId)
		c.Next()
	}
}

package middleware

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// NewAuthMiddleware returns a middleware that expects header "X-User-ID" with integer user id.
// Later this middleware will call Auth Service via gRPC to validate Authorization header.
func NewAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// For now: support X-User-ID for local testing
		if uid := c.GetHeader("X-User-ID"); uid != "" {
			if id, err := strconv.Atoi(uid); err == nil {
				c.Set("userID", id)
				c.Next()
				return
			}
		}

		// Fallback: try Authorization Bearer token - currently reject (until Auth gRPC implemented)
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authorization required (X-User-ID for local)"})
			return
		}

		// TODO: call Auth Service via gRPC to verify token and set userID
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token validation not implemented yet"})
	}
}

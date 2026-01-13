package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func InternalAuthMiddleware(apiKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		providedKey := c.GetHeader("X-Internal-API-Key")
		if providedKey == "" || providedKey != apiKey {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "invalid internal api key"})
			return
		}
		c.Next()
	}
}

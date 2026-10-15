package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"mystic-square/internal/auth"
	"mystic-square/internal/model"
)

// AuthMiddleware validates a Bearer JWT and stores its identity in Gin context.
func AuthMiddleware(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		parts := strings.Fields(c.GetHeader("Authorization"))
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, model.NewErrorResponse("unauthorized", "authentication required"))
			return
		}
		claims, err := auth.ParseToken(secret, parts[1], time.Now())
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, model.NewErrorResponse("unauthorized", "invalid or expired token"))
			return
		}
		c.Set("user_id", claims.UserID)
		c.Set("role", string(claims.Role))
		c.Next()
	}
}

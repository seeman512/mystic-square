package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"mystic-square/internal/model"
)

// RequireRole allows a request only when its authenticated role is listed.
// AuthMiddleware must run before this middleware.
func RequireRole(allowed ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		roleValue, exists := c.Get("role")
		role, ok := roleValue.(string)
		if !exists || !ok || role == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, model.NewErrorResponse("unauthorized", "authentication required"))
			return
		}
		for _, permitted := range allowed {
			if role == permitted {
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(http.StatusForbidden, model.NewErrorResponse("forbidden", "insufficient permissions"))
	}
}

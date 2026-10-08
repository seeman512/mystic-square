package handler

import (
	"github.com/gin-gonic/gin"

	"mystic-square/internal/model"
)

func writeError(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, model.NewErrorResponse(code, message))
}

package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"mystic-square/internal/service"
)

// UserHandler exposes the initial user operations.
type UserHandler struct {
	service *service.UserService
}

// NewUserHandler creates a user handler.
func NewUserHandler(userService *service.UserService) *UserHandler {
	return &UserHandler{service: userService}
}

// RegisterRoutes registers user routes on router.
func (h *UserHandler) RegisterRoutes(router *gin.RouterGroup) {
	users := router.Group("/users")
	users.POST("", h.Create)
	users.GET("", h.List)
}

// Create handles POST /users by creating a regular default user.
//
// @Summary Create a default user
// @Tags users
// @Produce json
// @Success 201 {object} model.User
// @Failure 500 {object} model.ErrorResponse
// @Router /users [post]
func (h *UserHandler) Create(c *gin.Context) {
	user, err := h.service.CreateDefault(c.Request.Context())
	if err != nil {
		handleUserServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, user)
}

// List handles GET /users.
//
// @Summary List users
// @Tags users
// @Produce json
// @Success 200 {array} model.User
// @Failure 500 {object} model.ErrorResponse
// @Router /users [get]
func (h *UserHandler) List(c *gin.Context) {
	users, err := h.service.List(c.Request.Context())
	if err != nil {
		handleUserServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, users)
}

func handleUserServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		writeError(c, http.StatusInternalServerError, "internal_server_error", "internal server error")
	case service.IsNotFound(err):
		writeError(c, http.StatusNotFound, "not_found", "user not found")
	default:
		writeError(c, http.StatusInternalServerError, "internal_server_error", "internal server error")
	}
}

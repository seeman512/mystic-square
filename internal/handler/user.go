package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"mystic-square/internal/model"
	"mystic-square/internal/service"
)

// UserHandler exposes administrative user operations.
type UserHandler struct {
	service *service.UserService
}

// NewUserHandler creates a user handler.
func NewUserHandler(userService *service.UserService) *UserHandler {
	return &UserHandler{service: userService}
}

// RegisterRoutes registers routes intended for administrator-only groups.
func (h *UserHandler) RegisterRoutes(router *gin.RouterGroup) {
	users := router.Group("/users")
	users.GET("", h.List)
	users.PUT("/:id/role", h.UpdateRole)
}

type updateUserRoleRequest struct {
	Role model.UserRole `json:"role"`
}

// List handles GET /users.
//
// @Summary List users
// @Tags users
// @Produce json
// @Success 200 {array} model.User
// @Failure 401 {object} model.ErrorResponse
// @Failure 403 {object} model.ErrorResponse
// @Failure 500 {object} model.ErrorResponse
// @Security BearerAuth
// @Router /users [get]
func (h *UserHandler) List(c *gin.Context) {
	users, err := h.service.List(c.Request.Context())
	if err != nil {
		handleUserServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, users)
}

// UpdateRole handles PUT /users/:id/role.
//
// @Summary Update a user's role
// @Tags users
// @Accept json
// @Produce json
// @Param id path int true "User ID"
// @Param input body updateUserRoleRequest true "Role update"
// @Success 200 {object} model.User
// @Failure 400 {object} model.ErrorResponse
// @Failure 401 {object} model.ErrorResponse
// @Failure 403 {object} model.ErrorResponse
// @Failure 404 {object} model.ErrorResponse
// @Failure 422 {object} model.ErrorResponse
// @Failure 500 {object} model.ErrorResponse
// @Security BearerAuth
// @Router /users/{id}/role [put]
func (h *UserHandler) UpdateRole(c *gin.Context) {
	id, ok := parseID(c.Param("id"))
	if !ok {
		writeError(c, http.StatusBadRequest, "invalid_id", "user id must be a positive integer")
		return
	}
	var input updateUserRoleRequest
	if err := decodeStrictJSON(c.Request.Body, &input); err != nil ||
		(input.Role != model.UserRoleAdmin && input.Role != model.UserRoleUser) {
		writeError(c, http.StatusUnprocessableEntity, "validation_error", "role must be admin or user")
		return
	}
	user, err := h.service.UpdateRole(c.Request.Context(), id, input.Role)
	if err != nil {
		handleUserServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, user)
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

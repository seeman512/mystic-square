package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"mystic-square/internal/auth"
	"mystic-square/internal/model"
	"mystic-square/internal/repository"
)

const (
	maxAuthRequestSize = 1 << 20

	registrationPayloadRules = "required fields: email (valid email address), full_name (1-200 characters), and password (8-72 characters); unknown fields are not allowed"
	loginPayloadRules        = "required fields: email (valid email address) and password (non-empty); unknown fields are not allowed"
)

// AuthHandler exposes public registration and login endpoints.
type AuthHandler struct {
	service *auth.Service
}

// NewAuthHandler creates an authentication handler.
func NewAuthHandler(authService *auth.Service) *AuthHandler {
	return &AuthHandler{service: authService}
}

// RegisterRoutes registers the public authentication endpoints.
func (h *AuthHandler) RegisterRoutes(router *gin.RouterGroup) {
	router.POST("/auth/register", h.Register)
	router.POST("/auth/login", h.Login)
}

// RegisterRequest contains only user-editable registration fields.
type RegisterRequest struct {
	Email    string `json:"email"`
	FullName string `json:"full_name"`
	Password string `json:"password"`
}

// LoginRequest contains the credentials required to authenticate.
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type authResponse struct {
	User  model.User `json:"user"`
	Token string     `json:"token"`
}

// Register handles POST /auth/register. The account role is always user.
//
// @Summary Register a user
// @Tags auth
// @Accept json
// @Produce json
// @Param input body RegisterRequest true "Registration payload"
// @Success 201 {object} authResponse
// @Failure 422 {object} model.ErrorResponse
// @Failure 500 {object} model.ErrorResponse
// @Router /auth/register [post]
func (h *AuthHandler) Register(c *gin.Context) {
	var input RegisterRequest
	if err := decodeStrictJSON(c.Request.Body, &input); err != nil {
		writeError(c, http.StatusUnprocessableEntity, "validation_error", authPayloadMessage("registration", err))
		return
	}
	user, token, err := h.service.Register(c.Request.Context(), input.Email, input.FullName, input.Password)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrInvalidRegistration):
			writeError(c, http.StatusUnprocessableEntity, "validation_error", authPayloadMessage("registration", nil))
		case errors.Is(err, repository.ErrConflict):
			writeError(c, http.StatusUnprocessableEntity, "validation_error", "email is already registered")
		case errors.Is(err, auth.ErrInvalidSecret):
			writeError(c, http.StatusInternalServerError, "internal_server_error", "authentication is not configured")
		default:
			writeError(c, http.StatusInternalServerError, "internal_server_error", "internal server error")
		}
		return
	}
	c.JSON(http.StatusCreated, authResponse{User: user, Token: token})
}

// Login handles POST /auth/login with a non-enumerating failure response.
//
// @Summary Login
// @Tags auth
// @Accept json
// @Produce json
// @Param input body LoginRequest true "Login credentials"
// @Success 200 {object} authResponse
// @Failure 401 {object} model.ErrorResponse
// @Failure 422 {object} model.ErrorResponse
// @Failure 500 {object} model.ErrorResponse
// @Router /auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var input LoginRequest
	if err := decodeStrictJSON(c.Request.Body, &input); err != nil {
		writeError(c, http.StatusUnprocessableEntity, "validation_error", authPayloadMessage("login", err))
		return
	}
	user, token, err := h.service.Login(c.Request.Context(), input.Email, input.Password)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrInvalidLogin):
			writeError(c, http.StatusUnprocessableEntity, "validation_error", authPayloadMessage("login", nil))
		case errors.Is(err, auth.ErrInvalidCredentials):
			writeError(c, http.StatusUnauthorized, "invalid_credentials", "invalid credentials")
		case errors.Is(err, auth.ErrInvalidSecret):
			writeError(c, http.StatusInternalServerError, "internal_server_error", "authentication is not configured")
		default:
			writeError(c, http.StatusInternalServerError, "internal_server_error", "internal server error")
		}
		return
	}
	c.JSON(http.StatusOK, authResponse{User: user, Token: token})
}

func decodeStrictJSON(reader io.Reader, value any) error {
	decoder := json.NewDecoder(io.LimitReader(reader, maxAuthRequestSize))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}

func authPayloadMessage(kind string, decodeErr error) string {
	base := "invalid " + kind + " payload"
	rules := registrationPayloadRules
	if kind == "login" {
		rules = loginPayloadRules
	}

	if decodeErr == nil {
		return base + ": " + rules
	}

	detail := "request body must be valid JSON"
	switch {
	case errors.Is(decodeErr, io.EOF):
		detail = "request body is required"
	case errors.Is(decodeErr, io.ErrUnexpectedEOF):
		detail = "request body must be complete valid JSON"
	case strings.Contains(decodeErr.Error(), "unknown field"):
		detail = "request contains an unknown field"
	case isJSONTypeError(decodeErr):
		detail = "request fields have invalid types"
	}
	return base + ": " + detail + "; " + rules
}

func isJSONTypeError(err error) bool {
	var typeErr *json.UnmarshalTypeError
	return errors.As(err, &typeErr)
}

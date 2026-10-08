package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"mystic-square/internal/model"
	"mystic-square/internal/service"
)

const maxLevelRequestSize = 1 << 20

// LevelHandler exposes level operations through HTTP.
type LevelHandler struct {
	service service.LevelService
}

// NewLevelHandler creates an HTTP handler for levelService.
func NewLevelHandler(levelService service.LevelService) *LevelHandler {
	return &LevelHandler{service: levelService}
}

// RegisterRoutes registers the level resource routes on router.
func (h *LevelHandler) RegisterRoutes(router *gin.RouterGroup) {
	levels := router.Group("/levels")
	levels.POST("", h.Create)
	levels.GET("", h.List)
	levels.GET("/:id", h.Get)
	levels.PUT("/:id", h.Update)
	levels.DELETE("/:id", h.Delete)
}

// Create handles POST /levels.
func (h *LevelHandler) Create(c *gin.Context) {
	input, err := decodeLevelInput(c.Request.Body)
	if err != nil {
		writeLevelValidationError(c, err)
		return
	}

	level, err := h.service.Create(c.Request.Context(), input)
	if err != nil {
		handleServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, level)
}

// Get handles GET /levels/:id.
func (h *LevelHandler) Get(c *gin.Context) {
	id, ok := parseID(c.Param("id"))
	if !ok {
		writeError(c, http.StatusBadRequest, "invalid_id", "level id must be a positive integer")
		return
	}

	level, err := h.service.Get(c.Request.Context(), id)
	if err != nil {
		handleServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, level)
}

// List handles GET /levels. It supports optional page, limit and difficulty parameters.
func (h *LevelHandler) List(c *gin.Context) {
	page, limit, ok := parsePagination(c)
	if !ok {
		writeError(c, http.StatusBadRequest, "invalid_pagination", "page and limit must be positive integers")
		return
	}

	levels, err := h.service.List(c.Request.Context())
	if err != nil {
		handleServiceError(c, err)
		return
	}

	if difficulty := c.Query("difficulty"); difficulty != "" {
		filtered := make([]model.Level, 0, len(levels))
		for _, level := range levels {
			if string(level.Difficulty) == difficulty {
				filtered = append(filtered, level)
			}
		}
		levels = filtered
	}

	start, end, ok := paginationBounds(page, limit, len(levels))
	if !ok {
		c.JSON(http.StatusOK, []model.Level{})
		return
	}
	c.JSON(http.StatusOK, levels[start:end])
}

// Update handles PUT /levels/:id.
func (h *LevelHandler) Update(c *gin.Context) {
	id, ok := parseID(c.Param("id"))
	if !ok {
		writeError(c, http.StatusBadRequest, "invalid_id", "level id must be a positive integer")
		return
	}

	input, err := decodeLevelInput(c.Request.Body)
	if err != nil {
		writeLevelValidationError(c, err)
		return
	}

	level, err := h.service.Update(c.Request.Context(), id, input)
	if err != nil {
		handleServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, level)
}

// Delete handles DELETE /levels/:id.
func (h *LevelHandler) Delete(c *gin.Context) {
	id, ok := parseID(c.Param("id"))
	if !ok {
		writeError(c, http.StatusBadRequest, "invalid_id", "level id must be a positive integer")
		return
	}

	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		handleServiceError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func handleServiceError(c *gin.Context, err error) {
	switch {
	case model.IsValidationError(err):
		writeLevelValidationError(c, err)
	case service.IsNotFound(err):
		writeError(c, http.StatusNotFound, "not_found", "level not found")
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		writeError(c, http.StatusInternalServerError, "internal_server_error", "internal server error")
	default:
		writeError(c, http.StatusInternalServerError, "internal_server_error", "internal server error")
	}
}

func parseID(raw string) (uint64, bool) {
	id, err := strconv.ParseUint(raw, 10, 64)
	return id, err == nil && id > 0
}

func parsePagination(c *gin.Context) (uint64, uint64, bool) {
	page := uint64(1)
	limit := uint64(10)
	var err error
	if value, exists := c.GetQuery("page"); exists {
		page, err = strconv.ParseUint(value, 10, 64)
		if err != nil || page == 0 {
			return 0, 0, false
		}
	}
	if value, exists := c.GetQuery("limit"); exists {
		limit, err = strconv.ParseUint(value, 10, 64)
		if err != nil || limit == 0 {
			return 0, 0, false
		}
	}
	if limit > 100 {
		limit = 100
	}
	return page, limit, true
}

func paginationBounds(page, limit uint64, length int) (int, int, bool) {
	if length == 0 || page-1 > uint64(length)/limit {
		return 0, 0, false
	}
	start := (page - 1) * limit
	if start >= uint64(length) {
		return 0, 0, false
	}
	end := start + limit
	if end > uint64(length) {
		end = uint64(length)
	}
	return int(start), int(end), true
}

type levelAPIError struct {
	Code    string             `json:"code"`
	Message string             `json:"message"`
	Fields  []model.FieldError `json:"fields,omitempty"`
}

type levelErrorResponse struct {
	Error levelAPIError `json:"error"`
}

func writeLevelValidationError(c *gin.Context, err error) {
	var validationErr *model.LevelValidationError
	message := levelValidationMessage(err)
	var fields []model.FieldError
	if errors.As(err, &validationErr) {
		message = "invalid level payload"
		fields = validationErr.Fields
	}

	c.AbortWithStatusJSON(http.StatusUnprocessableEntity, levelErrorResponse{
		Error: levelAPIError{
			Code:    "validation_error",
			Message: message,
			Fields:  fields,
		},
	})
}

func levelValidationMessage(err error) string {
	message := "invalid level payload"
	if err == nil {
		return message
	}

	var typeError *json.UnmarshalTypeError
	var syntaxError *json.SyntaxError
	switch {
	case errors.As(err, &typeError):
		field := typeError.Field
		if field == "" {
			return message + ": request body must be a JSON object"
		}
		return message + ": " + field + " has an invalid type; expected " + typeError.Type.String()
	case errors.Is(err, io.EOF):
		return message + ": request body is required"
	case errors.Is(err, io.ErrUnexpectedEOF), errors.As(err, &syntaxError):
		return message + ": request body must be valid JSON"
	}

	details := strings.ReplaceAll(err.Error(), "\n", "; ")
	return message + ": " + details
}

func decodeLevelInput(body io.Reader) (model.LevelInput, error) {
	decoder := json.NewDecoder(io.LimitReader(body, maxLevelRequestSize))
	var input model.LevelInput
	if err := decoder.Decode(&input); err != nil {
		return model.LevelInput{}, err
	}

	var extra json.RawMessage
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return model.LevelInput{}, errors.New("multiple json values")
		}
		return model.LevelInput{}, err
	}

	return input, nil
}

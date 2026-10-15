package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"mystic-square/internal/model"
	"mystic-square/internal/service"
)

// GameHandler exposes game creation, inspection, and moves.
type GameHandler struct {
	service *service.GameService
}

// NewGameHandler creates a game handler.
func NewGameHandler(gameService *service.GameService) *GameHandler {
	return &GameHandler{service: gameService}
}

// RegisterRoutes registers game routes on router.
func (h *GameHandler) RegisterRoutes(router *gin.RouterGroup) {
	games := router.Group("/games")
	games.POST("", h.Create)
	games.GET("", h.List)
	games.GET("/:id", h.Get)
	games.POST("/:id/move", h.Move)
	// Keep the plural spelling available to clients that model moves as a subresource.
	games.POST("/:id/moves", h.Move)
}

type createGameRequest struct {
	LevelID uint64 `json:"level_id"`
}

// Create handles POST /games. The request selects a level; the authenticated identity owns the game.
//
// @Summary Create a game
// @Tags games
// @Accept json
// @Produce json
// @Param input body createGameRequest true "Game selection"
// @Success 201 {object} model.Game
// @Failure 409 {object} model.ErrorResponse
// @Failure 422 {object} model.ErrorResponse
// @Router /games [post]
func (h *GameHandler) Create(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	var input createGameRequest
	if err := decodeStrictJSON(c.Request.Body, &input); err != nil {
		writeError(c, http.StatusUnprocessableEntity, "validation_error", "invalid game payload")
		return
	}
	if input.LevelID == 0 {
		writeError(c, http.StatusUnprocessableEntity, "validation_error", "level_id is required")
		return
	}

	game, err := h.service.Create(c.Request.Context(), userID, input.LevelID)
	if err != nil {
		handleGameServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, game)
}

// List handles GET /games.
//
// @Summary List games
// @Tags games
// @Produce json
// @Success 200 {array} model.Game
// @Failure 500 {object} model.ErrorResponse
// @Router /games [get]
func (h *GameHandler) List(c *gin.Context) {
	games, err := h.service.List(c.Request.Context())
	if err != nil {
		handleGameServiceError(c, err)
		return
	}
	if !isAdmin(c) {
		userID, ok := authenticatedUserID(c)
		if !ok {
			writeError(c, http.StatusUnauthorized, "unauthorized", "authentication required")
			return
		}
		owned := make([]model.Game, 0, len(games))
		for _, game := range games {
			if game.UserID == userID {
				owned = append(owned, game)
			}
		}
		games = owned
	}
	c.JSON(http.StatusOK, games)
}

// Get handles GET /games/:id.
//
// @Summary Get a game
// @Tags games
// @Produce json
// @Param id path int true "Game ID"
// @Success 200 {object} model.Game
// @Failure 400 {object} model.ErrorResponse
// @Failure 404 {object} model.ErrorResponse
// @Router /games/{id} [get]
func (h *GameHandler) Get(c *gin.Context) {
	id, ok := parseID(c.Param("id"))
	if !ok {
		writeError(c, http.StatusBadRequest, "invalid_id", "game id must be a positive integer")
		return
	}
	game, err := h.service.Get(c.Request.Context(), id)
	if err != nil {
		handleGameServiceError(c, err)
		return
	}
	if !canAccessGame(c, game) {
		writeError(c, http.StatusNotFound, "not_found", "game not found")
		return
	}
	c.JSON(http.StatusOK, game)
}

type moveRequest struct {
	Row    *int        `json:"row"`
	Column *int        `json:"column"`
	Move   *model.Move `json:"move"`
}

// Move handles POST /games/:id/move. Coordinates are zero-based.
//
// @Summary Apply a move
// @Description Coordinates are zero-based and must select a tile adjacent to the empty cell.
// @Tags games
// @Accept json
// @Produce json
// @Param id path int true "Game ID"
// @Param input body moveRequest true "Move coordinates"
// @Success 200 {object} model.Game
// @Failure 404 {object} model.ErrorResponse
// @Failure 422 {object} model.ErrorResponse
// @Router /games/{id}/move [post]
func (h *GameHandler) Move(c *gin.Context) {
	id, ok := parseID(c.Param("id"))
	if !ok {
		writeError(c, http.StatusBadRequest, "invalid_id", "game id must be a positive integer")
		return
	}

	game, err := h.service.Get(c.Request.Context(), id)
	if err != nil {
		handleGameServiceError(c, err)
		return
	}
	if !canAccessGame(c, game) {
		writeError(c, http.StatusNotFound, "not_found", "game not found")
		return
	}

	var input moveRequest
	if err := decodeJSON(c.Request.Body, &input); err != nil {
		writeError(c, http.StatusUnprocessableEntity, "validation_error", "row and column are required")
		return
	}
	var move model.Move
	switch {
	case input.Move != nil:
		move = *input.Move
	case input.Row != nil && input.Column != nil:
		move = model.Move{*input.Row, *input.Column}
	default:
		writeError(c, http.StatusUnprocessableEntity, "validation_error", "row and column are required")
		return
	}

	game, err = h.service.Move(c.Request.Context(), id, move)
	if err != nil {
		handleGameServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, game)
}

func authenticatedUserID(c *gin.Context) (uint64, bool) {
	value, exists := c.Get("user_id")
	if !exists {
		return 0, false
	}
	id, ok := value.(uint64)
	return id, ok && id > 0
}

func isAdmin(c *gin.Context) bool {
	return c.GetString("role") == string(model.UserRoleAdmin)
}

func canAccessGame(c *gin.Context, game model.Game) bool {
	if isAdmin(c) {
		return true
	}
	userID, ok := authenticatedUserID(c)
	return ok && userID == game.UserID
}

func handleGameServiceError(c *gin.Context, err error) {
	switch {
	case service.IsNotFound(err):
		writeError(c, http.StatusNotFound, "not_found", "game, level, or user not found")
	case service.IsConflict(err):
		writeError(c, http.StatusConflict, "conflict", "game already exists for this user and level")
	case service.IsInvalidMove(err):
		writeError(c, http.StatusUnprocessableEntity, "invalid_move", "move must select a tile adjacent to the empty cell")
	case errors.Is(err, service.ErrGameFinished):
		writeError(c, http.StatusConflict, "game_finished", "game has already been won")
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		writeError(c, http.StatusInternalServerError, "internal_server_error", "internal server error")
	default:
		writeError(c, http.StatusInternalServerError, "internal_server_error", "internal server error")
	}
}

func decodeJSON(reader io.Reader, value any) error {
	decoder := json.NewDecoder(io.LimitReader(reader, 1<<20))
	if err := decoder.Decode(value); err != nil {
		return err
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple json values")
		}
		return err
	}
	return nil
}

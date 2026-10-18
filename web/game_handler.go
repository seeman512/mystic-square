package web

import (
	"errors"
	"net/http"
	"strconv"

	"mystic-square/internal/model"
	"mystic-square/internal/repository"
	"mystic-square/internal/service"
)

func (h *Handler) game(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.methodNotAllowed(w, http.MethodGet)
		return
	}
	user, ok := h.currentUser(r)
	if !ok {
		h.expireSessionIfPresent(w, r)
		h.redirect(w, r, "/login")
		return
	}
	if h.levels == nil || h.games == nil {
		h.renderError(w, r, "Game", "game services are not configured", http.StatusInternalServerError)
		return
	}
	csrfToken := h.ensureCSRFToken(w, r)
	selectedID, err := queryID(r, "game_id")
	if err != nil {
		h.renderError(w, r, "Game", "invalid game id", http.StatusBadRequest)
		return
	}
	data, err := h.gamePageData(r, user, selectedID, "", "")
	data.CSRFToken = csrfToken
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			h.renderError(w, r, "Game", "game not found", http.StatusNotFound)
			return
		}
		h.logger.Error("load game page", "error", err)
		h.renderError(w, r, "Game", "we could not load the game page", http.StatusInternalServerError)
		return
	}
	if isHTMXRequest(r) && selectedID != 0 {
		h.renderNamedPage(w, r, "game", "game-response", data, http.StatusOK)
		return
	}
	h.renderPage(w, r, "game", data, http.StatusOK)
}

func (h *Handler) gameStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.methodNotAllowed(w, http.MethodPost)
		return
	}
	user, ok := h.currentUser(r)
	if !ok {
		h.expireSessionIfPresent(w, r)
		h.redirect(w, r, "/login")
		return
	}
	if h.levels == nil || h.games == nil {
		h.renderError(w, r, "Game", "game services are not configured", http.StatusInternalServerError)
		return
	}
	if err := parseGameForm(w, r); err != nil {
		h.renderGameActionError(w, r, user, 0, http.StatusUnprocessableEntity, "Please choose a valid level.")
		return
	}
	if !h.validCSRF(r) {
		h.renderGameActionError(w, r, user, 0, http.StatusForbidden, "Your session form has expired. Please refresh and try again.")
		return
	}
	levelID, err := parsePositiveID(r.FormValue("level_id"))
	if err != nil {
		h.renderGameActionError(w, r, user, 0, http.StatusUnprocessableEntity, "Please choose a valid level.")
		return
	}
	game, err := h.games.Create(r.Context(), user.ID, levelID)
	if err != nil {
		if service.IsConflict(err) {
			games, listErr := h.games.ListForUser(r.Context(), user.ID)
			if listErr == nil {
				for _, existing := range games {
					if existing.LevelID == levelID {
						data, dataErr := h.gamePageData(r, user, existing.ID, "A game for this level already exists, so it was loaded.", "")
						if dataErr != nil {
							h.renderGameServiceError(w, r, dataErr)
							return
						}
						h.renderGameAction(w, r, data, http.StatusOK, existing.ID)
						return
					}
				}
			}
		}
		status, message := gameServiceError(err)
		h.renderGameActionError(w, r, user, 0, status, message)
		return
	}
	data, err := h.gamePageData(r, user, game.ID, "", "")
	if err != nil {
		h.renderGameServiceError(w, r, err)
		return
	}
	h.renderGameAction(w, r, data, http.StatusOK, game.ID)
}

func (h *Handler) gameMove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.methodNotAllowed(w, http.MethodPost)
		return
	}
	user, ok := h.currentUser(r)
	if !ok {
		h.expireSessionIfPresent(w, r)
		h.redirect(w, r, "/login")
		return
	}
	id, err := gameIDFromPath(r.URL.Path)
	if err != nil {
		h.renderError(w, r, "Game", "invalid game id", http.StatusBadRequest)
		return
	}
	if h.levels == nil || h.games == nil {
		h.renderError(w, r, "Game", "game services are not configured", http.StatusInternalServerError)
		return
	}
	if err := parseGameForm(w, r); err != nil {
		h.renderGameActionError(w, r, user, id, http.StatusUnprocessableEntity, "Please choose a valid tile.")
		return
	}
	if !h.validCSRF(r) {
		h.renderGameActionError(w, r, user, id, http.StatusForbidden, "Your session form has expired. Please refresh and try again.")
		return
	}
	row, err := parseCoordinate(r.FormValue("row"))
	if err != nil {
		h.renderGameActionError(w, r, user, id, http.StatusUnprocessableEntity, "Please choose a valid tile.")
		return
	}
	column, err := parseCoordinate(r.FormValue("column"))
	if err != nil {
		h.renderGameActionError(w, r, user, id, http.StatusUnprocessableEntity, "Please choose a valid tile.")
		return
	}
	if _, err := h.games.MoveForUser(r.Context(), user.ID, id, model.Move{row, column}); err != nil {
		status, message := gameServiceError(err)
		h.renderGameActionError(w, r, user, id, status, message)
		return
	}
	data, err := h.gamePageData(r, user, id, "", "")
	if err != nil {
		h.renderGameServiceError(w, r, err)
		return
	}
	h.renderGameAction(w, r, data, http.StatusOK, id)
}

func (h *Handler) renderGameAction(w http.ResponseWriter, r *http.Request, data PageData, status int, gameID uint64) {
	if !isHTMXRequest(r) {
		h.redirect(w, r, "/game?game_id="+strconv.FormatUint(gameID, 10))
		return
	}
	h.renderNamedPage(w, r, "game", "game-response", data, status)
}

func (h *Handler) renderGameActionError(w http.ResponseWriter, r *http.Request, user model.User, gameID uint64, status int, message string) {
	data, err := h.gamePageData(r, user, gameID, "", message)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			h.renderError(w, r, "Game", "game not found", http.StatusNotFound)
			return
		}
		h.renderGameServiceError(w, r, err)
		return
	}
	if isHTMXRequest(r) {
		h.renderNamedPage(w, r, "game", "game-response", data, status)
		return
	}
	h.renderPage(w, r, "game", data, status)
}

func (h *Handler) renderGameServiceError(w http.ResponseWriter, r *http.Request, err error) {
	status, message := gameServiceError(err)
	if status == http.StatusNotFound {
		h.renderError(w, r, "Game", message, status)
		return
	}
	h.renderError(w, r, "Game", message, status)
}

func gameServiceError(err error) (int, string) {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		return http.StatusNotFound, "game not found"
	case service.IsConflict(err):
		return http.StatusConflict, "a game already exists for this level"
	case service.IsInvalidMove(err):
		return http.StatusUnprocessableEntity, "move that tile next to the empty cell"
	case errors.Is(err, service.ErrGameFinished):
		return http.StatusConflict, "this game is already completed"
	case service.IsGameOver(err):
		return http.StatusConflict, "game over: the move limit has been reached"
	default:
		return http.StatusInternalServerError, "we could not update the game"
	}
}

package web

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"mystic-square/internal/auth"
	"mystic-square/internal/model"
	"mystic-square/internal/repository"
	"mystic-square/internal/service"
)

const (
	// SessionCookieName is the name of the browser-only authentication cookie.
	SessionCookieName   = "mystic_square_session"
	CSRFTokenCookieName = "mystic_square_csrf"
	maxAuthBodySize     = 1 << 20
	dateFormat          = "2006-01-02"
)

// LevelLister lists levels available to the game page.
type LevelLister interface {
	List(context.Context) ([]model.Level, error)
}

// GamePageService contains the operations required by the authenticated game page.
type GamePageService interface {
	Create(context.Context, uint64, uint64) (model.Game, error)
	ListForUser(context.Context, uint64) ([]model.Game, error)
	GetForUser(context.Context, uint64, uint64) (model.Game, error)
	MoveForUser(context.Context, uint64, uint64, model.Move) (model.Game, error)
}

// Config controls the web authentication and rendering behavior.
type Config struct {
	// JWTSecret must be the same secret used by the JSON API.
	JWTSecret string
	// Levels and Games provide the services used by the authenticated game page.
	Levels LevelLister
	Games  GamePageService
	// SecureCookie should be true when the application is served over HTTPS.
	SecureCookie bool
	// Logger receives template and response-write errors. A nil logger uses
	// slog.Default.
	Logger *slog.Logger
	// Now is used for cookie expiry and the server-rendered footer date. A nil
	// value uses time.Now.
	Now func() time.Time
}

// PageData is the deliberately small view model exposed to the templates.
// It does not contain tokens, password hashes, or the complete User value.
type PageData struct {
	Title         string
	Date          string
	Authenticated bool
	FullName      string
	Email         string
	FullNameInput string
	Errors        []string
	Error         string
	IsLogin       bool
	IsRegister    bool
	IsGame        bool
	CSRFToken     string
	Levels        []LevelPage
	Games         []GameSummaryPage
	SelectedGame  *GamePage
	GameNotice    string
	GameError     string
}

// LevelPage is the safe level view exposed to the game template.
type LevelPage struct {
	ID              uint64
	Title           string
	Difficulty      string
	Rows            int
	Columns         int
	MoveLimit       int
	MoveLimitLabel  string
	ExistingGameID  uint64
	HasExistingGame bool
}

// GameSummaryPage is the safe summary view shown in the games panel.
type GameSummaryPage struct {
	ID          uint64
	LevelTitle  string
	Rows        int
	Columns     int
	Status      string
	Moves       int
	Elapsed     string
	Selected    bool
	CanContinue bool
}

// GamePage is the safe current-game view exposed to the game template.
type GamePage struct {
	ID         uint64
	LevelTitle string
	Difficulty string
	Rows       int
	Columns    int
	Moves      int
	MaxMoves   int
	Elapsed    string
	Status     string
	Message    string
	Error      string
	CanMove    bool
	BoardRows  [][]BoardCell
}

// BoardCell describes one rendered puzzle cell.
type BoardCell struct {
	Value     int
	Row       int
	Column    int
	Empty     bool
	Movable   bool
	AriaLabel string
}

// Handler serves the HTML pages and embedded static assets for the web UI.
type Handler struct {
	service       *auth.Service
	users         repository.UserRepository
	levels        LevelLister
	games         GamePageService
	jwtSecret     string
	secureCookie  bool
	logger        *slog.Logger
	now           func() time.Time
	mux           *http.ServeMux
	baseTemplates *template.Template
	pageTemplates map[string]*template.Template
}

// NewHandler creates the embedded-template web handler.
func NewHandler(service *auth.Service, users repository.UserRepository, config Config) (*Handler, error) {
	if service == nil {
		return nil, errors.New("web: authentication service is nil")
	}
	if users == nil {
		return nil, errors.New("web: user repository is nil")
	}
	logger := config.Logger
	if logger == nil {
		logger = slog.Default()
	}
	now := config.Now
	if now == nil {
		now = time.Now
	}

	base, err := template.ParseFS(TemplatesFS, "layout.html", "partials/header.html", "partials/footer.html")
	if err != nil {
		return nil, fmt.Errorf("parse shared web templates: %w", err)
	}
	pages := make(map[string]*template.Template, 4)
	for _, page := range []string{"login", "register", "game", "logout"} {
		pageTemplates, err := base.Clone()
		if err != nil {
			return nil, fmt.Errorf("clone templates for %s: %w", page, err)
		}
		pageTemplates, err = pageTemplates.ParseFS(TemplatesFS, page+".html")
		if err != nil {
			return nil, fmt.Errorf("parse %s template: %w", page, err)
		}
		pages[page] = pageTemplates
	}

	h := &Handler{
		service:       service,
		users:         users,
		levels:        config.Levels,
		games:         config.Games,
		jwtSecret:     config.JWTSecret,
		secureCookie:  config.SecureCookie,
		logger:        logger,
		now:           now,
		baseTemplates: base,
		pageTemplates: pages,
	}
	h.mux = http.NewServeMux()
	h.mux.HandleFunc("/login", h.login)
	h.mux.HandleFunc("/register", h.register)
	h.mux.HandleFunc("/game", h.game)
	h.mux.HandleFunc("/game/start", h.gameStart)
	h.mux.HandleFunc("/game/", h.gameMove)
	h.mux.HandleFunc("/logout", h.logout)
	h.mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(StaticFS))))
	return h, nil
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if _, ok := h.currentUser(r); ok {
			h.redirect(w, r, "/game")
			return
		}
		h.renderPage(w, r, "login", h.pageData("Login", r, PageData{IsLogin: true}), http.StatusOK)
	case http.MethodPost:
		h.handleLogin(w, r)
	default:
		h.methodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if _, ok := h.currentUser(r); ok {
			h.redirect(w, r, "/game")
			return
		}
		h.renderPage(w, r, "register", h.pageData("Register", r, PageData{IsRegister: true}), http.StatusOK)
	case http.MethodPost:
		h.handleRegister(w, r)
	default:
		h.methodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

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

func (h *Handler) gamePageData(r *http.Request, user model.User, selectedID uint64, notice, gameError string) (PageData, error) {
	levels, err := h.levels.List(r.Context())
	if err != nil {
		return PageData{}, fmt.Errorf("list levels for game page: %w", err)
	}
	games, err := h.games.ListForUser(r.Context(), user.ID)
	if err != nil {
		return PageData{}, fmt.Errorf("list user games for game page: %w", err)
	}
	var selected *model.Game
	if selectedID != 0 {
		loaded, loadErr := h.games.GetForUser(r.Context(), user.ID, selectedID)
		if loadErr != nil {
			return PageData{}, loadErr
		}
		selected = &loaded
		games, err = h.games.ListForUser(r.Context(), user.ID)
		if err != nil {
			return PageData{}, fmt.Errorf("refresh user games for game page: %w", err)
		}
	}

	levelByID := make(map[uint64]model.Level, len(levels))
	for _, level := range levels {
		levelByID[level.ID] = level
	}
	gameByLevel := make(map[uint64]model.Game, len(games))
	for _, game := range games {
		gameByLevel[game.LevelID] = game
	}
	data := h.pageData("Game", r, PageData{
		Authenticated: true,
		FullName:      user.FullName,
		IsGame:        true,
		CSRFToken:     csrfTokenFromRequest(r),
		GameNotice:    notice,
		GameError:     gameError,
	})
	data.Levels = make([]LevelPage, 0, len(levels))
	for _, level := range levels {
		limit := service.EffectiveMoveLimit(level)
		view := LevelPage{
			ID:             level.ID,
			Title:          level.Title,
			Difficulty:     string(level.Difficulty),
			Rows:           level.Rows,
			Columns:        level.Columns,
			MoveLimit:      limit,
			MoveLimitLabel: fmt.Sprintf("%d moves", limit),
		}
		if existing, ok := gameByLevel[level.ID]; ok {
			view.HasExistingGame = true
			view.ExistingGameID = existing.ID
		}
		data.Levels = append(data.Levels, view)
	}
	data.Games = make([]GameSummaryPage, 0, len(games))
	for _, game := range games {
		level := levelByID[game.LevelID]
		data.Games = append(data.Games, gameSummaryPage(game, level, selectedID, h.now()))
	}
	if selected != nil {
		level, ok := levelByID[selected.LevelID]
		if !ok {
			return PageData{}, fmt.Errorf("level %d for game %d is missing", selected.LevelID, selected.ID)
		}
		view, viewErr := gamePageView(*selected, level, h.now(), gameError)
		if viewErr != nil {
			return PageData{}, viewErr
		}
		data.SelectedGame = &view
	}
	return data, nil
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

func parseGameForm(w http.ResponseWriter, r *http.Request) error {
	if r.Body != nil {
		r.Body = http.MaxBytesReader(w, r.Body, maxAuthBodySize)
	}
	return r.ParseForm()
}

func parsePositiveID(value string) (uint64, error) {
	id, err := strconv.ParseUint(value, 10, 64)
	if err != nil || id == 0 {
		return 0, errors.New("invalid positive id")
	}
	return id, nil
}

func parseCoordinate(value string) (int, error) {
	coordinate, err := strconv.Atoi(value)
	if err != nil || coordinate < 0 {
		return 0, errors.New("invalid coordinate")
	}
	return coordinate, nil
}

func queryID(r *http.Request, key string) (uint64, error) {
	value := r.URL.Query().Get(key)
	if value == "" {
		return 0, nil
	}
	return parsePositiveID(value)
}

func gameIDFromPath(path string) (uint64, error) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 3 || parts[0] != "game" || parts[2] != "move" {
		return 0, errors.New("invalid game route")
	}
	return parsePositiveID(parts[1])
}

func (h *Handler) ensureCSRFToken(w http.ResponseWriter, r *http.Request) string {
	if token := csrfTokenFromRequest(r); token != "" {
		return token
	}
	bytesValue := make([]byte, 32)
	if _, err := rand.Read(bytesValue); err != nil {
		h.logger.Error("generate csrf token", "error", err)
		return ""
	}
	token := base64.RawURLEncoding.EncodeToString(bytesValue)
	http.SetCookie(w, &http.Cookie{
		Name:     CSRFTokenCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.secureCookie,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(auth.TokenLifetime / time.Second),
		Expires:  h.now().UTC().Add(auth.TokenLifetime),
	})
	return token
}

func csrfTokenFromRequest(r *http.Request) string {
	cookie, err := r.Cookie(CSRFTokenCookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}

func (h *Handler) validCSRF(r *http.Request) bool {
	cookieToken := csrfTokenFromRequest(r)
	if cookieToken == "" {
		return false
	}
	if r.PostFormValue("csrf_token") != "" {
		formToken := r.PostFormValue("csrf_token")
		return subtle.ConstantTimeCompare([]byte(cookieToken), []byte(formToken)) == 1
	}
	headerToken := r.Header.Get("X-CSRF-Token")
	return subtle.ConstantTimeCompare([]byte(cookieToken), []byte(headerToken)) == 1
}

func gameStatus(game model.Game) string {
	if game.IsWin {
		return "Completed"
	}
	if game.IsOver {
		return "Game over"
	}
	return "In progress"
}

func gameSummaryPage(game model.Game, level model.Level, selectedID uint64, now time.Time) GameSummaryPage {
	status := gameStatus(game)
	return GameSummaryPage{
		ID:          game.ID,
		LevelTitle:  level.Title,
		Rows:        level.Rows,
		Columns:     level.Columns,
		Status:      status,
		Moves:       len(game.Moves),
		Elapsed:     elapsedTime(game, now),
		Selected:    game.ID == selectedID,
		CanContinue: status == "In progress",
	}
}

func gamePageView(game model.Game, level model.Level, now time.Time, message string) (GamePage, error) {
	if len(game.Board) != level.Rows*level.Columns {
		return GamePage{}, fmt.Errorf("game %d has an invalid board", game.ID)
	}
	empty := -1
	for index, value := range game.Board {
		if value == 0 {
			empty = index
			break
		}
	}
	if empty < 0 {
		return GamePage{}, fmt.Errorf("game %d has no empty cell", game.ID)
	}
	status := gameStatus(game)
	view := GamePage{
		ID:         game.ID,
		LevelTitle: level.Title,
		Difficulty: string(level.Difficulty),
		Rows:       level.Rows,
		Columns:    level.Columns,
		Moves:      len(game.Moves),
		MaxMoves:   service.EffectiveMoveLimit(level),
		Elapsed:    elapsedTime(game, now),
		Status:     status,
		Message:    message,
		CanMove:    status == "In progress",
		BoardRows:  make([][]BoardCell, level.Rows),
	}
	for row := 0; row < level.Rows; row++ {
		view.BoardRows[row] = make([]BoardCell, level.Columns)
		for column := 0; column < level.Columns; column++ {
			index := row*level.Columns + column
			value := game.Board[index]
			movable := empty >= 0 && absInt(row-empty/level.Columns)+absInt(column-empty%level.Columns) == 1
			view.BoardRows[row][column] = BoardCell{
				Value:     value,
				Row:       row,
				Column:    column,
				Empty:     value == 0,
				Movable:   movable && value != 0 && view.CanMove,
				AriaLabel: boardCellLabel(value, row, column),
			}
		}
	}
	return view, nil
}

func boardCellLabel(value, row, column int) string {
	if value == 0 {
		return fmt.Sprintf("Empty cell, row %d, column %d", row+1, column+1)
	}
	return fmt.Sprintf("Tile %d, row %d, column %d", value, row+1, column+1)
}

func elapsedTime(game model.Game, now time.Time) string {
	start := game.StartedAt
	if start.IsZero() {
		start = now
	}
	end := now
	if game.CompletedAt != nil {
		end = *game.CompletedAt
	}
	if end.Before(start) {
		end = start
	}
	duration := end.Sub(start).Round(time.Second)
	hours := int(duration / time.Hour)
	duration -= time.Duration(hours) * time.Hour
	minutes := int(duration / time.Minute)
	duration -= time.Duration(minutes) * time.Minute
	seconds := int(duration / time.Second)
	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// GET is intentionally idempotent and does not clear state. The POST
		// action has already expired the cookie before redirecting here.
		h.renderPage(w, r, "logout", h.pageData("Goodbye", r, PageData{}), http.StatusOK)
	case http.MethodPost:
		h.expireSession(w)
		h.redirect(w, r, "/logout")
	default:
		h.methodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

func (h *Handler) handleLogin(w http.ResponseWriter, r *http.Request) {
	email, password, err := loginPayload(w, r)
	if err != nil {
		h.renderFormError(w, r, "login", PageData{IsLogin: true}, http.StatusUnprocessableEntity,
			"Please enter a valid email and password.")
		return
	}

	email = strings.TrimSpace(email)
	_, token, err := h.service.Login(r.Context(), email, password)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrInvalidLogin), errors.Is(err, auth.ErrInvalidCredentials):
			h.renderFormError(w, r, "login", PageData{
				IsLogin: true,
				Email:   email,
			}, http.StatusUnprocessableEntity, "Invalid email or password")
		case errors.Is(err, auth.ErrInvalidSecret):
			h.renderFormError(w, r, "login", PageData{IsLogin: true, Email: email}, http.StatusInternalServerError,
				"Authentication is not configured. Please try again later.")
		default:
			h.logger.Error("web login failed", "error", err)
			h.renderFormError(w, r, "login", PageData{IsLogin: true, Email: email}, http.StatusInternalServerError,
				"We could not sign you in. Please try again later.")
		}
		return
	}

	h.setSession(w, token)
	h.redirect(w, r, "/game")
}

func (h *Handler) handleRegister(w http.ResponseWriter, r *http.Request) {
	email, fullName, password, err := registerPayload(w, r)
	if err != nil {
		h.renderFormError(w, r, "register", PageData{IsRegister: true}, http.StatusUnprocessableEntity,
			"Please provide a valid name, email, and password.")
		return
	}

	email = strings.TrimSpace(email)
	fullName = strings.TrimSpace(fullName)
	_, token, err := h.service.Register(r.Context(), email, fullName, password)
	if err != nil {
		data := PageData{
			IsRegister:    true,
			Email:         email,
			FullNameInput: fullName,
		}
		switch {
		case errors.Is(err, auth.ErrInvalidRegistration):
			h.renderFormError(w, r, "register", data, http.StatusUnprocessableEntity,
				"Please provide a valid name, email, and password (passwords must be 8-72 characters).")
		case errors.Is(err, repository.ErrConflict):
			h.renderFormError(w, r, "register", data, http.StatusUnprocessableEntity,
				"An account with that email already exists.")
		case errors.Is(err, auth.ErrInvalidSecret):
			h.renderFormError(w, r, "register", data, http.StatusInternalServerError,
				"Authentication is not configured. Please try again later.")
		default:
			h.logger.Error("web registration failed", "error", err)
			h.renderFormError(w, r, "register", data, http.StatusInternalServerError,
				"We could not create your account. Please try again later.")
		}
		return
	}

	h.setSession(w, token)
	h.redirect(w, r, "/game")
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type registerRequest struct {
	Email    string `json:"email"`
	FullName string `json:"full_name"`
	Password string `json:"password"`
}

func loginPayload(w http.ResponseWriter, r *http.Request) (string, string, error) {
	if isJSONRequest(r) {
		var input loginRequest
		if err := decodeJSONBody(w, r, &input); err != nil {
			return "", "", err
		}
		return input.Email, input.Password, nil
	}

	values, err := parseForm(w, r)
	if err != nil {
		return "", "", err
	}
	return values.Get("email"), values.Get("password"), nil
}

func registerPayload(w http.ResponseWriter, r *http.Request) (string, string, string, error) {
	if isJSONRequest(r) {
		var input registerRequest
		if err := decodeJSONBody(w, r, &input); err != nil {
			return "", "", "", err
		}
		return input.Email, input.FullName, input.Password, nil
	}

	values, err := parseForm(w, r)
	if err != nil {
		return "", "", "", err
	}
	return values.Get("email"), values.Get("full_name"), values.Get("password"), nil
}

func isJSONRequest(r *http.Request) bool {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	return err == nil && mediaType == "application/json"
}

func decodeJSONBody(w http.ResponseWriter, r *http.Request, value any) error {
	if r.Body == nil {
		return io.EOF
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxAuthBodySize))
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

func parseForm(w http.ResponseWriter, r *http.Request) (mapValues, error) {
	if r.Body != nil {
		r.Body = http.MaxBytesReader(w, r.Body, maxAuthBodySize)
	}
	if err := r.ParseForm(); err != nil {
		return nil, err
	}
	return mapValues(r.Form), nil
}

// mapValues keeps the handler independent of the mutable Request.Form map and
// exposes only the read operation needed by the authentication forms.
type mapValues map[string][]string

func (values mapValues) Get(key string) string {
	items := values[key]
	if len(items) == 0 {
		return ""
	}
	return items[0]
}

func (h *Handler) pageData(title string, r *http.Request, data PageData) PageData {
	data.Title = title
	data.Date = h.now().UTC().Format(dateFormat)
	if !data.Authenticated {
		// Login, register, and logout are deliberately guest pages. The only
		// pages that opt into the authenticated header do so explicitly above.
		data.FullName = ""
	}
	return data
}

func (h *Handler) renderFormError(w http.ResponseWriter, r *http.Request, page string, data PageData, status int, message string) {
	data.Errors = []string{message}
	h.renderPage(w, r, page, h.pageData(pageTitle(page), r, data), status)
}

func pageTitle(page string) string {
	if page == "register" {
		return "Register"
	}
	return "Login"
}

func (h *Handler) renderPage(w http.ResponseWriter, r *http.Request, page string, data PageData, status int) {
	name := "layout"
	if isHTMXRequest(r) && (page == "login" || page == "register") && status >= http.StatusUnprocessableEntity {
		name = page + "-form"
	}
	h.renderNamedPage(w, r, page, name, data, status)
}

func (h *Handler) renderNamedPage(w http.ResponseWriter, r *http.Request, page, name string, data PageData, status int) {
	// Both full-page and fragment responses can be returned by these routes, so
	// caches must distinguish them by HX-Request.
	w.Header().Add("Vary", "HX-Request")

	tmpl := h.pageTemplates[page]
	if tmpl == nil {
		h.logger.Error("web template is missing", "page", page)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	var body bytes.Buffer
	if err := tmpl.ExecuteTemplate(&body, name, data); err != nil {
		h.logger.Error("execute web template", "page", page, "template", name, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if _, err := io.Copy(w, &body); err != nil {
		h.logger.Error("write web response", "page", page, "error", err)
	}
}

func (h *Handler) renderError(w http.ResponseWriter, r *http.Request, title, message string, status int) {
	w.Header().Add("Vary", "HX-Request")
	data := h.pageData(title, r, PageData{Error: message})
	var body bytes.Buffer
	if err := h.baseTemplates.ExecuteTemplate(&body, "layout", data); err != nil {
		h.logger.Error("execute web error template", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if _, err := io.Copy(w, &body); err != nil {
		h.logger.Error("write web error response", "error", err)
	}
}

func (h *Handler) currentUser(r *http.Request) (model.User, bool) {
	cookie, err := r.Cookie(SessionCookieName)
	if err != nil {
		return model.User{}, false
	}
	claims, err := auth.ParseToken(h.jwtSecret, cookie.Value, h.now())
	if err != nil {
		return model.User{}, false
	}
	user, err := h.users.Get(r.Context(), claims.UserID)
	if err != nil {
		return model.User{}, false
	}
	return user, true
}

func (h *Handler) setSession(w http.ResponseWriter, token string) {
	now := h.now().UTC()
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.secureCookie,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(auth.TokenLifetime / time.Second),
		Expires:  now.Add(auth.TokenLifetime),
	})
}

func (h *Handler) expireSession(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   h.secureCookie,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
		Expires:  time.Unix(1, 0).UTC(),
	})
}

func (h *Handler) expireSessionIfPresent(w http.ResponseWriter, r *http.Request) {
	if _, err := r.Cookie(SessionCookieName); err == nil {
		h.expireSession(w)
	}
}

func (h *Handler) redirect(w http.ResponseWriter, r *http.Request, location string) {
	w.Header().Add("Vary", "HX-Request")
	if isHTMXRequest(r) {
		w.Header().Set("HX-Redirect", location)
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, location, http.StatusSeeOther)
}

func isHTMXRequest(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("HX-Request"), "true")
}

func (h *Handler) methodNotAllowed(w http.ResponseWriter, methods ...string) {
	w.Header().Set("Allow", strings.Join(methods, ", "))
	h.renderError(w, &http.Request{}, "Method Not Allowed", "method not allowed", http.StatusMethodNotAllowed)
}

var _ http.Handler = (*Handler)(nil)

package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"mystic-square/internal/auth"
	"mystic-square/internal/model"
	"mystic-square/internal/repository"
	"mystic-square/internal/service"
)

func TestHandler_GamePageStartsAndMovesOwnedGame(t *testing.T) {
	users := repository.NewInMemoryUserRepository()
	levels := repository.NewInMemoryLevelRepository()
	games := repository.NewInMemoryGameRepository()
	levelService := service.NewLevelService(levels)
	rows, columns, moveLimit := 3, 3, 10
	level, err := levelService.Create(t.Context(), model.LevelInput{
		Title: pointerForGameTest("Starter"), Theme: pointerForGameTest("classic"), Objective: pointerForGameTest("solve"),
		Difficulty: pointerForGameTest(model.DifficultyLow), Rows: &rows, Columns: &columns, MoveLimit: &moveLimit,
	})
	if err != nil {
		t.Fatal(err)
	}
	gameService := service.NewGameService(games, levels, users)
	authService := auth.NewService(users, testSecret)
	h, err := NewHandler(authService, users, Config{
		JWTSecret: testSecret,
		Levels:    levelService,
		Games:     gameService,
		Now: func() time.Time {
			return time.Date(2026, time.October, 5, 12, 0, 0, 0, time.UTC)
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	register := httptest.NewRecorder()
	h.ServeHTTP(register, formRequest(http.MethodPost, "/register", url.Values{
		"email":     {"game@example.test"},
		"full_name": {"Game Player"},
		"password":  {"correct-horse-42"},
	}))
	if register.Code != http.StatusSeeOther || register.Header().Get("Location") != "/game" {
		t.Fatalf("registration = %d %q", register.Code, register.Header().Get("Location"))
	}
	sessionCookie := register.Result().Cookies()[0]

	pageRequest := httptest.NewRequest(http.MethodGet, "/game", nil)
	pageRequest.AddCookie(sessionCookie)
	page := httptest.NewRecorder()
	h.ServeHTTP(page, pageRequest)
	if page.Code != http.StatusOK {
		t.Fatalf("game page = %d, want 200: %s", page.Code, page.Body.String())
	}
	if !strings.Contains(page.Body.String(), "Start Game") || !strings.Contains(page.Body.String(), "Starter") {
		t.Fatalf("game page does not contain the level panel: %s", page.Body.String())
	}
	csrfCookie := page.Result().Cookies()[0]
	csrfToken := csrfCookie.Value

	startRequest := httptest.NewRequest(http.MethodPost, "/game/start", strings.NewReader(url.Values{
		"csrf_token": {csrfToken},
		"level_id":   {"1"},
	}.Encode()))
	startRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	startRequest.AddCookie(sessionCookie)
	startRequest.AddCookie(csrfCookie)
	start := httptest.NewRecorder()
	h.ServeHTTP(start, startRequest)
	if start.Code != http.StatusSeeOther || !strings.HasPrefix(start.Header().Get("Location"), "/game?game_id=") {
		t.Fatalf("start response = %d %q", start.Code, start.Header().Get("Location"))
	}

	game, err := gameService.ListForUser(t.Context(), 1)
	if err != nil || len(game) != 1 || game[0].LevelID != level.ID {
		t.Fatalf("created games = %+v, error = %v", game, err)
	}
	zero := -1
	for index, value := range game[0].Board {
		if value == 0 {
			zero = index
			break
		}
	}
	if zero < 0 {
		t.Fatal("created game has no empty cell")
	}
	row, column := zero/columns, zero%columns
	if column > 0 {
		column--
	} else if column+1 < columns {
		column++
	} else {
		row--
	}

	moveRequest := httptest.NewRequest(http.MethodPost, "/game/1/move", strings.NewReader(url.Values{
		"csrf_token": {csrfToken},
		"row":        {strconv.Itoa(row)},
		"column":     {strconv.Itoa(column)},
	}.Encode()))
	moveRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	moveRequest.AddCookie(sessionCookie)
	moveRequest.AddCookie(csrfCookie)
	move := httptest.NewRecorder()
	h.ServeHTTP(move, moveRequest)
	if move.Code != http.StatusSeeOther || !strings.HasPrefix(move.Header().Get("Location"), "/game?game_id=1") {
		t.Fatalf("move response = %d %q: %s", move.Code, move.Header().Get("Location"), move.Body.String())
	}
}

func pointerForGameTest[T any](value T) *T {
	return &value
}

package app_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestUsersAndGames(t *testing.T) {
	e := newEnv(t)

	userResponse := e.do(http.MethodPost, "/api/v1/users", "")
	e.expect(userResponse, http.StatusCreated)
	userID := idOf(t, userResponse.obj(t))

	level := map[string]any{
		"title":      "Three by three",
		"theme":      "classic",
		"objective":  "solve",
		"difficulty": "medium",
		"rows":       3,
		"columns":    3,
	}
	levelResponse := e.doJSON(http.MethodPost, "/api/v1/levels", level)
	e.expect(levelResponse, http.StatusCreated)
	levelID := idOf(t, levelResponse.obj(t))

	gameResponse := e.doJSON(http.MethodPost, "/api/v1/games", map[string]any{
		"user_id":  userID,
		"level_id": levelID,
	})
	e.expect(gameResponse, http.StatusCreated)
	game := gameResponse.obj(t)
	if got, ok := game["board"].([]any); !ok || len(got) != 9 {
		t.Fatalf("game board = %v, want nine cells", game["board"])
	}
	if got := len(game["moves"].([]any)); got != 0 {
		t.Fatalf("new game has %d moves, want 0", got)
	}

	board := game["board"].([]any)
	zero := -1
	for index, value := range board {
		if value.(float64) == 0 {
			zero = index
			break
		}
	}
	if zero < 0 {
		t.Fatal("new game has no empty cell")
	}
	tile := zero + 1
	if zero%3 > 0 {
		tile = zero - 1
	}
	moveResponse := e.doJSON(http.MethodPost, "/api/v1/games/1/move", map[string]any{
		"row":    tile / 3,
		"column": tile % 3,
	})
	e.expect(moveResponse, http.StatusOK)
	var moved map[string]any
	if err := json.Unmarshal(moveResponse.Body, &moved); err != nil {
		t.Fatal(err)
	}
	if got := len(moved["moves"].([]any)); got != 1 {
		t.Fatalf("moved game has %d moves, want 1", got)
	}

	users := e.do(http.MethodGet, "/api/v1/users", "")
	e.expect(users, http.StatusOK)
	if got := len(users.list(t)); got != 1 {
		t.Fatalf("user list has %d users, want 1", got)
	}
}

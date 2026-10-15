package app_test

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"
)

func TestUsersAndGames(t *testing.T) {
	e := newEnv(t)

	registerResponse := e.doJSON(http.MethodPost, "/api/v1/auth/register", map[string]any{
		"email": "player@example.test", "full_name": "Test Player", "password": "safe-password-123",
	})
	e.expect(registerResponse, http.StatusCreated)
	registered := registerResponse.obj(t)
	userID := idOf(t, registered["user"].(map[string]any))
	userToken := registered["token"].(string)

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

	gameResponse := e.doWithToken(http.MethodPost, "/api/v1/games", `{"level_id":`+strconv.Itoa(levelID)+`}`, userToken)
	e.expect(gameResponse, http.StatusCreated)
	game := gameResponse.obj(t)

	forgedOwner := e.doWithToken(http.MethodPost, "/api/v1/games", `{"user_id":1,"level_id":`+strconv.Itoa(levelID)+`}`, userToken)
	e.expect(forgedOwner, http.StatusUnprocessableEntity)

	otherResponse := e.doJSON(http.MethodPost, "/api/v1/auth/register", map[string]any{
		"email": "other-player@example.test", "full_name": "Other Player", "password": "correct-horse-42",
	})
	e.expect(otherResponse, http.StatusCreated)
	otherToken := otherResponse.obj(t)["token"].(string)
	deniedGame := e.doWithToken(http.MethodGet, "/api/v1/games/1", "", otherToken)
	e.expect(deniedGame, http.StatusNotFound)
	otherGames := e.doWithToken(http.MethodGet, "/api/v1/games", "", otherToken)
	e.expect(otherGames, http.StatusOK)
	if got := len(otherGames.list(t)); got != 0 {
		t.Fatalf("other user's game list has %d entries, want 0", got)
	}
	if got, ok := game["board"].([]any); !ok || len(got) != 9 {
		t.Fatalf("game board = %v, want nine cells", game["board"])
	}
	if got := game["user_id"].(float64); got != float64(userID) {
		t.Fatalf("new game user_id = %v, want authenticated user %d", got, userID)
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
	moveBody := `{"row":` + strconv.Itoa(tile/3) + `,"column":` + strconv.Itoa(tile%3) + `}`
	moveResponse := e.doWithToken(http.MethodPost, "/api/v1/games/1/move", moveBody, userToken)
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
	if got := len(users.list(t)); got != 3 {
		t.Fatalf("user list has %d users, want admin and two registered players", got)
	}
}

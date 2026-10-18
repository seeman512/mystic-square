package service

import (
	"errors"
	"testing"

	"mystic-square/internal/model"
	"mystic-square/internal/repository"
)

func TestGameServiceMoveMarksGameOverAtMoveLimit(t *testing.T) {
	users := repository.NewInMemoryUserRepository()
	levels := repository.NewInMemoryLevelRepository()
	games := repository.NewInMemoryGameRepository()

	user, err := users.Create(t.Context(), model.User{Email: "player@example.test", FullName: "Player", Role: model.UserRoleUser})
	if err != nil {
		t.Fatal(err)
	}
	moveLimit := 1
	level, err := levels.Create(t.Context(), model.Level{
		Title: "Two by two", Theme: "classic", Objective: "solve", Difficulty: model.DifficultyLow,
		Rows: 2, Columns: 2, MoveLimit: &moveLimit,
	})
	if err != nil {
		t.Fatal(err)
	}
	game, err := games.Create(t.Context(), model.Game{
		LevelID: level.ID,
		UserID:  user.ID,
		Init:    []int{0, 1, 3, 2},
		Moves:   []model.Move{},
	})
	if err != nil {
		t.Fatal(err)
	}

	service := NewGameService(games, levels, users)
	moved, err := service.Move(t.Context(), game.ID, model.Move{0, 1})
	if err != nil {
		t.Fatalf("Move() error = %v", err)
	}
	if !moved.IsOver || moved.IsWin || len(moved.Moves) != 1 {
		t.Fatalf("moved game = %+v, want a one-move game over", moved)
	}

	if _, err := service.Move(t.Context(), game.ID, model.Move{0, 1}); !errors.Is(err, ErrGameOver) {
		t.Fatalf("second Move() error = %v, want ErrGameOver", err)
	}
}

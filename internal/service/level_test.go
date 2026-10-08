package service

import (
	"context"
	"testing"

	"mystic-square/internal/model"
	"mystic-square/internal/repository"
)

func TestLevelServiceCreateSetsTimestamps(t *testing.T) {
	service := NewLevelService(repository.NewInMemoryLevelRepository())
	title := "Starter"
	difficulty := model.DifficultyMedium
	rows, columns, timeLimit, moveLimit := 4, 4, -1, -1
	level, err := service.Create(context.Background(), model.LevelInput{
		Title:      &title,
		Difficulty: &difficulty,
		Rows:       &rows,
		Columns:    &columns,
		TimeLimit:  &timeLimit,
		MoveLimit:  &moveLimit,
	})
	if err != nil {
		t.Fatal(err)
	}
	if level.CreatedAt.IsZero() {
		t.Fatal("service must set the creation timestamp")
	}
}

package service

import (
	"context"
	"testing"

	"mystic-square/internal/model"
	"mystic-square/internal/repository"
)

func TestLevelServiceUpdateTitleDescriptionPreservesConfiguration(t *testing.T) {
	repository := repository.NewInMemoryLevelRepository()
	levelService := NewLevelService(repository)
	title := "Original"
	theme := "forest"
	objective := "find the exit"
	difficulty := model.DifficultyHard
	description := "original description"
	rows, columns, timeLimit, moveLimit := 3, 5, 60, 20
	created, err := levelService.Create(context.Background(), model.LevelInput{
		Title: &title, Theme: &theme, Objective: &objective, Difficulty: &difficulty,
		Description: &description, Rows: &rows, Columns: &columns,
		TimeLimit: &timeLimit, MoveLimit: &moveLimit,
	})
	if err != nil {
		t.Fatal(err)
	}

	updatedTitle := "Updated"
	updatedDescription := "updated description"
	updated, err := levelService.UpdateTitleDescription(context.Background(), created.ID, updatedTitle, &updatedDescription)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Title != updatedTitle || updated.Description == nil || *updated.Description != updatedDescription {
		t.Fatalf("updated fields = %+v", updated)
	}
	if updated.Theme != created.Theme || updated.Objective != created.Objective || updated.Difficulty != created.Difficulty ||
		updated.Rows != created.Rows || updated.Columns != created.Columns || updated.TimeLimit == nil || *updated.TimeLimit != *created.TimeLimit ||
		updated.MoveLimit == nil || *updated.MoveLimit != *created.MoveLimit {
		t.Fatalf("level configuration changed: before=%+v after=%+v", created, updated)
	}
}

func TestLevelServiceCreateSetsTimestamps(t *testing.T) {
	service := NewLevelService(repository.NewInMemoryLevelRepository())
	title := "Starter"
	theme := "forest"
	objective := "find the exit"
	difficulty := model.DifficultyMedium
	rows, columns, timeLimit, moveLimit := 4, 4, -1, -1
	level, err := service.Create(context.Background(), model.LevelInput{
		Title:      &title,
		Theme:      &theme,
		Objective:  &objective,
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

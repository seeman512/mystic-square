package repository

import (
	"context"
	"testing"

	"mystic-square/internal/model"
)

func TestInMemoryLevelRepositoryIDsAndCopies(t *testing.T) {
	repo := NewInMemoryLevelRepository()
	description := "instructions"
	timeLimit, moveLimit := 60, 20
	created, err := repo.Create(context.Background(), model.Level{
		Title: "Starter", Description: &description, TimeLimit: &timeLimit, MoveLimit: &moveLimit,
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.ID != 1 {
		t.Fatalf("created ID = %d, want 1", created.ID)
	}
	*created.Description = "changed outside repository"
	*created.TimeLimit = 1
	*created.MoveLimit = 1

	got, err := repo.Get(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if *got.Description != description {
		t.Fatalf("repository returned aliased description: %q", *got.Description)
	}
	if got.TimeLimit == nil || *got.TimeLimit != timeLimit {
		t.Fatalf("repository returned aliased time limit: %v", got.TimeLimit)
	}
	if got.MoveLimit == nil || *got.MoveLimit != moveLimit {
		t.Fatalf("repository returned aliased move limit: %v", got.MoveLimit)
	}
	if err := repo.Delete(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	created, err = repo.Create(context.Background(), model.Level{Title: "Second"})
	if err != nil {
		t.Fatal(err)
	}
	if created.ID != 2 {
		t.Fatalf("created ID after delete = %d, want 2", created.ID)
	}
}

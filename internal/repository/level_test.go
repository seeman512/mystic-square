package repository

import (
	"context"
	"testing"

	"mystic-square/internal/model"
)

func TestInMemoryLevelRepositoryIDsAndCopies(t *testing.T) {
	repo := NewInMemoryLevelRepository()
	description := "instructions"
	created, err := repo.Create(context.Background(), model.Level{Title: "Starter", Description: &description})
	if err != nil {
		t.Fatal(err)
	}
	if created.ID != 1 {
		t.Fatalf("created ID = %d, want 1", created.ID)
	}
	*created.Description = "changed outside repository"

	got, err := repo.Get(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if *got.Description != description {
		t.Fatalf("repository returned aliased data: %q", *got.Description)
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

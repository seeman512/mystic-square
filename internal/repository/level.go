package repository

import (
	"context"
	"errors"
	"sync"

	"mystic-square/internal/model"
)

// ErrNotFound is returned when a level does not exist.
var ErrNotFound = errors.New("level not found")

// ErrIDExhausted is returned when no new level identifier can be assigned.
var ErrIDExhausted = errors.New("level id space exhausted")

// LevelRepository stores levels.
type LevelRepository interface {
	Create(ctx context.Context, level model.Level) (model.Level, error)
	Get(ctx context.Context, id uint64) (model.Level, error)
	List(ctx context.Context) ([]model.Level, error)
	Update(ctx context.Context, level model.Level) (model.Level, error)
	Delete(ctx context.Context, id uint64) error
}

// InMemoryLevelRepository is a concurrency-safe level repository.
type InMemoryLevelRepository struct {
	mu     sync.RWMutex
	nextID uint64
	items  map[uint64]model.Level
}

// NewInMemoryLevelRepository creates an empty level repository.
func NewInMemoryLevelRepository() *InMemoryLevelRepository {
	return &InMemoryLevelRepository{
		nextID: 1,
		items:  make(map[uint64]model.Level),
	}
}

// Create stores level and assigns its next unused identifier.
func (r *InMemoryLevelRepository) Create(ctx context.Context, level model.Level) (model.Level, error) {
	if err := contextError(ctx); err != nil {
		return model.Level{}, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.nextID == 0 {
		return model.Level{}, ErrIDExhausted
	}
	level.ID = r.nextID
	r.nextID++
	r.items[level.ID] = cloneLevel(level)
	return cloneLevel(level), nil
}

// Get returns a copy of the level with id.
func (r *InMemoryLevelRepository) Get(ctx context.Context, id uint64) (model.Level, error) {
	if err := contextError(ctx); err != nil {
		return model.Level{}, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	level, ok := r.items[id]
	if !ok {
		return model.Level{}, ErrNotFound
	}
	return cloneLevel(level), nil
}

// List returns independent copies ordered by identifier.
func (r *InMemoryLevelRepository) List(ctx context.Context) ([]model.Level, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	levels := make([]model.Level, 0, len(r.items))
	for id := uint64(1); id < r.nextID; id++ {
		if level, ok := r.items[id]; ok {
			levels = append(levels, cloneLevel(level))
		}
	}
	return levels, nil
}

// Update replaces an existing level without changing its identifier.
func (r *InMemoryLevelRepository) Update(ctx context.Context, level model.Level) (model.Level, error) {
	if err := contextError(ctx); err != nil {
		return model.Level{}, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.items[level.ID]; !ok {
		return model.Level{}, ErrNotFound
	}
	r.items[level.ID] = cloneLevel(level)
	return cloneLevel(level), nil
}

// Delete removes a level. Identifiers are deliberately not reused.
func (r *InMemoryLevelRepository) Delete(ctx context.Context, id uint64) error {
	if err := contextError(ctx); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.items[id]; !ok {
		return ErrNotFound
	}
	delete(r.items, id)
	return nil
}

func contextError(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	return ctx.Err()
}

func cloneLevel(level model.Level) model.Level {
	if level.Description != nil {
		description := *level.Description
		level.Description = &description
	}
	return level
}

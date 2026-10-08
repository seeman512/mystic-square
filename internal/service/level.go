package service

import (
	"context"
	"errors"
	"time"

	"mystic-square/internal/model"
	"mystic-square/internal/repository"
)

// LevelService contains level business operations.
type LevelService interface {
	Create(ctx context.Context, input model.LevelInput) (model.Level, error)
	Get(ctx context.Context, id uint64) (model.Level, error)
	List(ctx context.Context) ([]model.Level, error)
	Update(ctx context.Context, id uint64, input model.LevelInput) (model.Level, error)
	Delete(ctx context.Context, id uint64) error
}

// LevelServiceImpl validates level data and coordinates repository operations.
type LevelServiceImpl struct {
	repository repository.LevelRepository
	clock      func() time.Time
}

// NewLevelService creates a level service using repository.
func NewLevelService(repository repository.LevelRepository) *LevelServiceImpl {
	return &LevelServiceImpl{
		repository: repository,
		clock:      func() time.Time { return time.Now().UTC() },
	}
}

// Create validates and stores a new level.
func (s *LevelServiceImpl) Create(ctx context.Context, input model.LevelInput) (model.Level, error) {
	if err := input.Validate(); err != nil {
		return model.Level{}, err
	}

	now := s.clock().UTC()
	return s.repository.Create(ctx, model.Level{
		Title:       *input.Title,
		Description: input.Description,
		Difficulty:  *input.Difficulty,
		Columns:     *input.Columns,
		Rows:        *input.Rows,
		TimeLimit:   *input.TimeLimit,
		MoveLimit:   *input.MoveLimit,
		CreatedAt:   now,
	})
}

// Get returns a level by identifier.
func (s *LevelServiceImpl) Get(ctx context.Context, id uint64) (model.Level, error) {
	return s.repository.Get(ctx, id)
}

// List returns all levels ordered by identifier.
func (s *LevelServiceImpl) List(ctx context.Context) ([]model.Level, error) {
	return s.repository.List(ctx)
}

// Update validates and replaces a level while retaining its identifier and creation time.
func (s *LevelServiceImpl) Update(ctx context.Context, id uint64, input model.LevelInput) (model.Level, error) {
	if err := input.Validate(); err != nil {
		return model.Level{}, err
	}

	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return model.Level{}, err
	}
	current.Title = *input.Title
	current.Description = input.Description
	current.Difficulty = *input.Difficulty
	current.Columns = *input.Columns
	current.Rows = *input.Rows
	current.TimeLimit = *input.TimeLimit
	current.MoveLimit = *input.MoveLimit
	return s.repository.Update(ctx, current)
}

// Delete removes a level.
func (s *LevelServiceImpl) Delete(ctx context.Context, id uint64) error {
	return s.repository.Delete(ctx, id)
}

// IsNotFound reports whether err represents a missing level.
func IsNotFound(err error) bool {
	return errors.Is(err, repository.ErrNotFound)
}

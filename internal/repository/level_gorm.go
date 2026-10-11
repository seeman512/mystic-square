package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"mystic-square/internal/model"
)

// GORMLevelRepository stores levels in PostgreSQL through GORM.
type GORMLevelRepository struct {
	db *gorm.DB
}

// NewGORMLevelRepository creates a level repository backed by db.
func NewGORMLevelRepository(db *gorm.DB) *GORMLevelRepository {
	return &GORMLevelRepository{db: db}
}

// Create inserts a level and returns the row with its generated ID.
func (r *GORMLevelRepository) Create(ctx context.Context, level model.Level) (model.Level, error) {
	if r == nil || r.db == nil {
		return model.Level{}, errors.New("level repository database is nil")
	}
	if err := r.db.WithContext(repositoryContext(ctx)).Create(&level).Error; err != nil {
		return model.Level{}, fmt.Errorf("create level: %w", err)
	}
	return level, nil
}

// Get returns a level by ID.
func (r *GORMLevelRepository) Get(ctx context.Context, id uint64) (model.Level, error) {
	if r == nil || r.db == nil {
		return model.Level{}, errors.New("level repository database is nil")
	}
	var level model.Level
	err := r.db.WithContext(repositoryContext(ctx)).First(&level, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.Level{}, ErrNotFound
	}
	if err != nil {
		return model.Level{}, fmt.Errorf("get level %d: %w", id, err)
	}
	return level, nil
}

// List returns all levels in ascending ID order.
func (r *GORMLevelRepository) List(ctx context.Context) ([]model.Level, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("level repository database is nil")
	}
	var levels []model.Level
	if err := r.db.WithContext(repositoryContext(ctx)).Order("id ASC").Find(&levels).Error; err != nil {
		return nil, fmt.Errorf("list levels: %w", err)
	}
	if levels == nil {
		levels = []model.Level{}
	}
	return levels, nil
}

// Update replaces an existing level without changing its ID.
func (r *GORMLevelRepository) Update(ctx context.Context, level model.Level) (model.Level, error) {
	if r == nil || r.db == nil {
		return model.Level{}, errors.New("level repository database is nil")
	}
	result := r.db.WithContext(repositoryContext(ctx)).Model(&model.Level{}).Where("id = ?", level.ID).
		Select("title", "theme", "objective", "description", "difficulty", "columns", "rows", "time_limit", "move_limit", "updated_at").
		Updates(&level)
	if result.Error != nil {
		return model.Level{}, fmt.Errorf("update level %d: %w", level.ID, result.Error)
	}
	if result.RowsAffected == 0 {
		return model.Level{}, ErrNotFound
	}
	return level, nil
}

// Delete removes a level by ID.
func (r *GORMLevelRepository) Delete(ctx context.Context, id uint64) error {
	if r == nil || r.db == nil {
		return errors.New("level repository database is nil")
	}
	result := r.db.WithContext(repositoryContext(ctx)).Delete(&model.Level{}, id)
	if result.Error != nil {
		return fmt.Errorf("delete level %d: %w", id, result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func repositoryContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

var _ LevelRepository = (*GORMLevelRepository)(nil)

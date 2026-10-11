package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"mystic-square/internal/model"
)

// GameRepository stores game sessions.
type GameRepository interface {
	Create(ctx context.Context, game model.Game) (model.Game, error)
	Get(ctx context.Context, id uint64) (model.Game, error)
	List(ctx context.Context) ([]model.Game, error)
	Update(ctx context.Context, game model.Game) (model.Game, error)
	Delete(ctx context.Context, id uint64) error
}

// GORMGameRepository stores games in PostgreSQL through GORM.
type GORMGameRepository struct {
	db *gorm.DB
}

// NewGORMGameRepository creates a game repository backed by db.
func NewGORMGameRepository(db *gorm.DB) *GORMGameRepository {
	return &GORMGameRepository{db: db}
}

// Create inserts a game. The database unique constraint enforces one game per
// user and level pair.
func (r *GORMGameRepository) Create(ctx context.Context, game model.Game) (model.Game, error) {
	if err := r.checkDB(); err != nil {
		return model.Game{}, err
	}
	var count int64
	if err := r.db.WithContext(repositoryContext(ctx)).Model(&model.Game{}).
		Where("user_id = ? AND level_id = ?", game.UserID, game.LevelID).Count(&count).Error; err != nil {
		return model.Game{}, fmt.Errorf("check existing game: %w", err)
	}
	if count > 0 {
		return model.Game{}, ErrConflict
	}
	if err := r.db.WithContext(repositoryContext(ctx)).Create(&game).Error; err != nil {
		return model.Game{}, fmt.Errorf("create game: %w", err)
	}
	return game, nil
}

// Get returns a game with its level and user preloaded.
func (r *GORMGameRepository) Get(ctx context.Context, id uint64) (model.Game, error) {
	if err := r.checkDB(); err != nil {
		return model.Game{}, err
	}
	var game model.Game
	err := r.db.WithContext(repositoryContext(ctx)).Preload("Level").Preload("User").First(&game, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.Game{}, ErrNotFound
	}
	if err != nil {
		return model.Game{}, fmt.Errorf("get game %d: %w", id, err)
	}
	return game, nil
}

// List returns games with their level and user preloaded in a constant number
// of queries. The preloads avoid an N+1 query pattern in API list handlers.
func (r *GORMGameRepository) List(ctx context.Context) ([]model.Game, error) {
	if err := r.checkDB(); err != nil {
		return nil, err
	}
	var games []model.Game
	query := r.db.WithContext(repositoryContext(ctx)).Preload("Level").Preload("User").Order("id ASC")
	if err := query.Find(&games).Error; err != nil {
		return nil, fmt.Errorf("list games: %w", err)
	}
	if games == nil {
		games = []model.Game{}
	}
	return games, nil
}

// Update persists a game without changing its ID or ownership.
func (r *GORMGameRepository) Update(ctx context.Context, game model.Game) (model.Game, error) {
	if err := r.checkDB(); err != nil {
		return model.Game{}, err
	}
	result := r.db.WithContext(repositoryContext(ctx)).Model(&model.Game{}).Where("id = ?", game.ID).
		Select("level_id", "user_id", "init", "moves", "is_win").
		Updates(&game)
	if result.Error != nil {
		return model.Game{}, fmt.Errorf("update game %d: %w", game.ID, result.Error)
	}
	if result.RowsAffected == 0 {
		return model.Game{}, ErrNotFound
	}
	return game, nil
}

// Delete removes a game by ID.
func (r *GORMGameRepository) Delete(ctx context.Context, id uint64) error {
	if err := r.checkDB(); err != nil {
		return err
	}
	result := r.db.WithContext(repositoryContext(ctx)).Delete(&model.Game{}, id)
	if result.Error != nil {
		return fmt.Errorf("delete game %d: %w", id, result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *GORMGameRepository) checkDB() error {
	if r == nil || r.db == nil {
		return errors.New("game repository database is nil")
	}
	return nil
}

var _ GameRepository = (*GORMGameRepository)(nil)

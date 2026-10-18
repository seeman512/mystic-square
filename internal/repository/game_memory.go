package repository

import (
	"context"
	"errors"
	"sort"
	"sync"

	"mystic-square/internal/model"
)

// ErrConflict is returned when an operation violates a uniqueness rule.
var ErrConflict = errors.New("resource conflict")

// InMemoryGameRepository is a concurrency-safe game repository.
type InMemoryGameRepository struct {
	mu     sync.RWMutex
	nextID uint64
	items  map[uint64]model.Game
}

// NewInMemoryGameRepository creates an empty game repository.
func NewInMemoryGameRepository() *InMemoryGameRepository {
	return &InMemoryGameRepository{nextID: 1, items: make(map[uint64]model.Game)}
}

// Create stores a game and assigns its next unused identifier.
func (r *InMemoryGameRepository) Create(ctx context.Context, game model.Game) (model.Game, error) {
	if err := contextError(ctx); err != nil {
		return model.Game{}, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	for _, existing := range r.items {
		if existing.UserID == game.UserID && existing.LevelID == game.LevelID {
			return model.Game{}, ErrConflict
		}
	}
	if r.nextID == 0 {
		return model.Game{}, ErrIDExhausted
	}
	game.ID = r.nextID
	r.nextID++
	r.items[game.ID] = cloneGame(game)
	return cloneGame(game), nil
}

// Get returns a copy of the game with id.
func (r *InMemoryGameRepository) Get(ctx context.Context, id uint64) (model.Game, error) {
	if err := contextError(ctx); err != nil {
		return model.Game{}, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()
	game, ok := r.items[id]
	if !ok {
		return model.Game{}, ErrNotFound
	}
	return cloneGame(game), nil
}

// List returns independent copies ordered by identifier.
func (r *InMemoryGameRepository) List(ctx context.Context) ([]model.Game, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()
	games := make([]model.Game, 0, len(r.items))
	for id := uint64(1); id < r.nextID; id++ {
		if game, ok := r.items[id]; ok {
			games = append(games, cloneGame(game))
		}
	}
	return games, nil
}

// ListByUser returns only games owned by userID in game-page order.
func (r *InMemoryGameRepository) ListByUser(ctx context.Context, userID uint64) ([]model.Game, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()
	games := make([]model.Game, 0)
	for _, game := range r.items {
		if game.UserID == userID {
			games = append(games, cloneGame(game))
		}
	}
	sort.SliceStable(games, func(i, j int) bool {
		iTerminal := games[i].IsWin || games[i].IsOver
		jTerminal := games[j].IsWin || games[j].IsOver
		if iTerminal != jTerminal {
			return !iTerminal
		}
		if !games[i].LastAccessedAt.Equal(games[j].LastAccessedAt) {
			return games[i].LastAccessedAt.After(games[j].LastAccessedAt)
		}
		return games[i].ID > games[j].ID
	})
	return games, nil
}

// Update replaces the mutable state of a game without changing its ID.
func (r *InMemoryGameRepository) Update(ctx context.Context, game model.Game) (model.Game, error) {
	if err := contextError(ctx); err != nil {
		return model.Game{}, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.items[game.ID]; !ok {
		return model.Game{}, ErrNotFound
	}
	r.items[game.ID] = cloneGame(game)
	return cloneGame(game), nil
}

// Delete removes a game by ID.
func (r *InMemoryGameRepository) Delete(ctx context.Context, id uint64) error {
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

func cloneGame(game model.Game) model.Game {
	if game.Init != nil {
		game.Init = append(make([]int, 0, len(game.Init)), game.Init...)
	}
	if game.Moves != nil {
		game.Moves = append(make([]model.Move, 0, len(game.Moves)), game.Moves...)
	}
	if game.Board != nil {
		game.Board = append(make([]int, 0, len(game.Board)), game.Board...)
	}
	if game.CompletedAt != nil {
		completedAt := *game.CompletedAt
		game.CompletedAt = &completedAt
	}
	return game
}

var _ GameRepository = (*InMemoryGameRepository)(nil)

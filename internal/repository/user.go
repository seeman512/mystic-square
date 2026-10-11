package repository

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"gorm.io/gorm"

	"mystic-square/internal/model"
)

// UserRepository stores users.
type UserRepository interface {
	Create(ctx context.Context, user model.User) (model.User, error)
	List(ctx context.Context) ([]model.User, error)
	Get(ctx context.Context, id uint64) (model.User, error)
}

// InMemoryUserRepository is a concurrency-safe user repository.
type InMemoryUserRepository struct {
	mu     sync.RWMutex
	nextID uint64
	items  map[uint64]model.User
}

// NewInMemoryUserRepository creates an empty user repository.
func NewInMemoryUserRepository() *InMemoryUserRepository {
	return &InMemoryUserRepository{nextID: 1, items: make(map[uint64]model.User)}
}

// Create stores a user and assigns its next unused identifier.
func (r *InMemoryUserRepository) Create(ctx context.Context, user model.User) (model.User, error) {
	if err := contextError(ctx); err != nil {
		return model.User{}, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.nextID == 0 {
		return model.User{}, ErrIDExhausted
	}
	user.ID = r.nextID
	r.nextID++
	r.items[user.ID] = user
	return user, nil
}

// List returns users in ascending identifier order.
func (r *InMemoryUserRepository) List(ctx context.Context) ([]model.User, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()
	users := make([]model.User, 0, len(r.items))
	for id := uint64(1); id < r.nextID; id++ {
		if user, ok := r.items[id]; ok {
			users = append(users, user)
		}
	}
	return users, nil
}

// Get returns the user with id.
func (r *InMemoryUserRepository) Get(ctx context.Context, id uint64) (model.User, error) {
	if err := contextError(ctx); err != nil {
		return model.User{}, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()
	user, ok := r.items[id]
	if !ok {
		return model.User{}, ErrNotFound
	}
	return user, nil
}

// GORMUserRepository stores users in PostgreSQL through GORM.
type GORMUserRepository struct {
	db *gorm.DB
}

// NewGORMUserRepository creates a user repository backed by db.
func NewGORMUserRepository(db *gorm.DB) *GORMUserRepository {
	return &GORMUserRepository{db: db}
}

// Create inserts a user and returns the row with its generated ID.
func (r *GORMUserRepository) Create(ctx context.Context, user model.User) (model.User, error) {
	if err := r.checkDB(); err != nil {
		return model.User{}, err
	}
	if err := r.db.WithContext(repositoryContext(ctx)).Create(&user).Error; err != nil {
		return model.User{}, fmt.Errorf("create user: %w", err)
	}
	return user, nil
}

// List returns users in ascending identifier order.
func (r *GORMUserRepository) List(ctx context.Context) ([]model.User, error) {
	if err := r.checkDB(); err != nil {
		return nil, err
	}
	var users []model.User
	if err := r.db.WithContext(repositoryContext(ctx)).Order("id ASC").Find(&users).Error; err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	if users == nil {
		users = []model.User{}
	}
	return users, nil
}

// Get returns a user by ID.
func (r *GORMUserRepository) Get(ctx context.Context, id uint64) (model.User, error) {
	if err := r.checkDB(); err != nil {
		return model.User{}, err
	}
	var user model.User
	err := r.db.WithContext(repositoryContext(ctx)).First(&user, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.User{}, ErrNotFound
	}
	if err != nil {
		return model.User{}, fmt.Errorf("get user %d: %w", id, err)
	}
	return user, nil
}

func (r *GORMUserRepository) checkDB() error {
	if r == nil || r.db == nil {
		return errors.New("user repository database is nil")
	}
	return nil
}

var _ UserRepository = (*InMemoryUserRepository)(nil)
var _ UserRepository = (*GORMUserRepository)(nil)

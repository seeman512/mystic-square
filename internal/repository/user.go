package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"

	"mystic-square/internal/model"
)

// UserRepository stores users and supports authentication lookups.
type UserRepository interface {
	Create(ctx context.Context, user model.User) (model.User, error)
	List(ctx context.Context) ([]model.User, error)
	Get(ctx context.Context, id uint64) (model.User, error)
	GetByEmail(ctx context.Context, email string) (model.User, error)
	UpdateRole(ctx context.Context, id uint64, role model.UserRole) (model.User, error)
}

// InMemoryUserRepository is a concurrency-safe user repository.
type InMemoryUserRepository struct {
	mu       sync.RWMutex
	nextID   uint64
	items    map[uint64]model.User
	emailIDs map[string]uint64
}

// NewInMemoryUserRepository creates an empty user repository.
func NewInMemoryUserRepository() *InMemoryUserRepository {
	return &InMemoryUserRepository{
		nextID:   1,
		items:    make(map[uint64]model.User),
		emailIDs: make(map[string]uint64),
	}
}

// Create stores a user and assigns its next unused identifier.
func (r *InMemoryUserRepository) Create(ctx context.Context, user model.User) (model.User, error) {
	if err := contextError(ctx); err != nil {
		return model.User{}, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	email := strings.ToLower(user.Email)
	if email != "" {
		if _, exists := r.emailIDs[email]; exists {
			return model.User{}, ErrConflict
		}
	}
	if r.nextID == 0 {
		return model.User{}, ErrIDExhausted
	}
	user.ID = r.nextID
	r.nextID++
	r.items[user.ID] = user
	if email != "" {
		r.emailIDs[email] = user.ID
	}
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

// GetByEmail returns the user with email.
func (r *InMemoryUserRepository) GetByEmail(ctx context.Context, email string) (model.User, error) {
	if err := contextError(ctx); err != nil {
		return model.User{}, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()
	id, ok := r.emailIDs[strings.ToLower(email)]
	if !ok {
		return model.User{}, ErrNotFound
	}
	return r.items[id], nil
}

// UpdateRole changes the role assigned to a user.
func (r *InMemoryUserRepository) UpdateRole(ctx context.Context, id uint64, role model.UserRole) (model.User, error) {
	if err := contextError(ctx); err != nil {
		return model.User{}, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	user, ok := r.items[id]
	if !ok {
		return model.User{}, ErrNotFound
	}
	user.Role = role
	user.UpdatedAt = time.Now().UTC()
	r.items[id] = user
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
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return model.User{}, ErrConflict
		}
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

// GetByEmail returns a user by normalized email address.
func (r *GORMUserRepository) GetByEmail(ctx context.Context, email string) (model.User, error) {
	if err := r.checkDB(); err != nil {
		return model.User{}, err
	}
	var user model.User
	err := r.db.WithContext(repositoryContext(ctx)).Where("email = ?", strings.ToLower(email)).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.User{}, ErrNotFound
	}
	if err != nil {
		return model.User{}, fmt.Errorf("get user by email: %w", err)
	}
	return user, nil
}

// UpdateRole changes the role assigned to a user.
func (r *GORMUserRepository) UpdateRole(ctx context.Context, id uint64, role model.UserRole) (model.User, error) {
	if err := r.checkDB(); err != nil {
		return model.User{}, err
	}
	result := r.db.WithContext(repositoryContext(ctx)).Model(&model.User{}).Where("id = ?", id).
		Update("role", role)
	if result.Error != nil {
		return model.User{}, fmt.Errorf("update user %d role: %w", id, result.Error)
	}
	if result.RowsAffected == 0 {
		return model.User{}, ErrNotFound
	}
	return r.Get(ctx, id)
}

func (r *GORMUserRepository) checkDB() error {
	if r == nil || r.db == nil {
		return errors.New("user repository database is nil")
	}
	return nil
}

var _ UserRepository = (*InMemoryUserRepository)(nil)
var _ UserRepository = (*GORMUserRepository)(nil)

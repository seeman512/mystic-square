package service

import (
	"context"
	"time"

	"mystic-square/internal/model"
	"mystic-square/internal/repository"
)

const defaultUserName = "Default User"

// UserService provides the small user API used before authentication exists.
type UserService struct {
	repository repository.UserRepository
	clock      func() time.Time
}

// NewUserService creates a user service using repository.
func NewUserService(userRepository repository.UserRepository) *UserService {
	return &UserService{
		repository: userRepository,
		clock:      func() time.Time { return time.Now().UTC() },
	}
}

// CreateDefault creates a regular user with the system default name.
func (s *UserService) CreateDefault(ctx context.Context) (model.User, error) {
	now := s.clock().UTC()
	return s.repository.Create(ctx, model.User{
		FullName:  defaultUserName,
		Role:      model.UserRoleUser,
		CreatedAt: now,
		UpdatedAt: now,
	})
}

// List returns all users in repository order.
func (s *UserService) List(ctx context.Context) ([]model.User, error) {
	return s.repository.List(ctx)
}

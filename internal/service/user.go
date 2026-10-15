package service

import (
	"context"
	"fmt"

	"mystic-square/internal/model"
	"mystic-square/internal/repository"
)

// UserService provides administrative user operations.
type UserService struct {
	repository repository.UserRepository
}

// NewUserService creates a user service using userRepository.
func NewUserService(userRepository repository.UserRepository) *UserService {
	return &UserService{repository: userRepository}
}

// List returns all users in repository order.
func (s *UserService) List(ctx context.Context) ([]model.User, error) {
	return s.repository.List(ctx)
}

// UpdateRole changes the role of the user with id.
func (s *UserService) UpdateRole(ctx context.Context, id uint64, role model.UserRole) (model.User, error) {
	if role != model.UserRoleAdmin && role != model.UserRoleUser {
		return model.User{}, fmt.Errorf("unsupported user role %q", role)
	}
	return s.repository.UpdateRole(ctx, id, role)
}

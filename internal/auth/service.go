// Package auth implements account registration, password verification, and JWTs.
package auth

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"mystic-square/internal/model"
	"mystic-square/internal/repository"
)

// TokenLifetime is the validity period for issued authentication tokens.
const TokenLifetime = 24 * time.Hour

const minPasswordLength = 8

var (
	// ErrInvalidCredentials is returned for either an unknown email or bad password.
	ErrInvalidCredentials = errors.New("invalid credentials")
	// ErrInvalidRegistration indicates invalid user-supplied registration data.
	ErrInvalidRegistration = errors.New("invalid registration")
	// ErrInvalidSecret indicates that JWT_SECRET is missing or too short.
	ErrInvalidSecret = errors.New("JWT_SECRET must contain at least 32 bytes")
)

// Service handles user authentication. Its signing secret is provided by the
// application from the JWT_SECRET environment variable.
type Service struct {
	users  repository.UserRepository
	secret string
	now    func() time.Time
}

// NewService creates an authentication service using users and secret.
func NewService(users repository.UserRepository, secret string) *Service {
	return &Service{users: users, secret: secret, now: func() time.Time { return time.Now().UTC() }}
}

// Register validates and creates a regular user, then returns a signed token.
func (s *Service) Register(ctx context.Context, email, fullName, password string) (model.User, string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	fullName = strings.TrimSpace(fullName)
	parsedEmail, err := mail.ParseAddress(email)
	if err != nil || parsedEmail.Address != email || fullName == "" || len(fullName) > 200 ||
		len(password) < minPasswordLength || len(password) > 72 {
		return model.User{}, "", ErrInvalidRegistration
	}
	if err := validateSecret(s.secret); err != nil {
		return model.User{}, "", err
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return model.User{}, "", fmt.Errorf("hash password: %w", err)
	}
	now := s.now()
	user, err := s.users.Create(ctx, model.User{
		Email:        email,
		FullName:     fullName,
		PasswordHash: string(passwordHash),
		Role:         model.UserRoleUser,
		CreatedAt:    now,
		UpdatedAt:    now,
	})
	if err != nil {
		return model.User{}, "", err
	}
	token, err := SignToken(s.secret, user, now.Add(TokenLifetime))
	if err != nil {
		return model.User{}, "", err
	}
	return user, token, nil
}

// Login verifies credentials and returns a signed token. Unknown users and
// password mismatches intentionally produce the same error.
func (s *Service) Login(ctx context.Context, email, password string) (model.User, string, error) {
	if err := validateSecret(s.secret); err != nil {
		return model.User{}, "", err
	}
	email = strings.ToLower(strings.TrimSpace(email))
	user, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return model.User{}, "", ErrInvalidCredentials
		}
		return model.User{}, "", fmt.Errorf("lookup login user: %w", err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return model.User{}, "", ErrInvalidCredentials
	}
	token, err := SignToken(s.secret, user, s.now().Add(TokenLifetime))
	if err != nil {
		return model.User{}, "", err
	}
	return user, token, nil
}

func validateSecret(secret string) error {
	if len([]byte(secret)) < 32 {
		return ErrInvalidSecret
	}
	return nil
}

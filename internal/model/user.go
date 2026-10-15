package model

import "time"

// UserRole identifies the permissions assigned to a user.
type UserRole string

const (
	// UserRoleAdmin can create and modify levels.
	UserRoleAdmin UserRole = "admin"
	// UserRoleUser can play games but cannot modify levels.
	UserRoleUser UserRole = "user"
)

// User is an authenticated Mystic Square player or administrator.
type User struct {
	ID           uint64    `json:"id" gorm:"primaryKey" example:"1"`
	Email        string    `json:"email" gorm:"not null;uniqueIndex" example:"ada@example.com"`
	FullName     string    `json:"full_name" gorm:"not null" example:"Ada Lovelace"`
	PasswordHash string    `json:"-" gorm:"column:password_hash;not null"`
	Role         UserRole  `json:"role" gorm:"not null" example:"user"`
	CreatedAt    time.Time `json:"created_at" gorm:"not null"`
	UpdatedAt    time.Time `json:"updated_at" gorm:"not null"`
}

// CanManageLevels reports whether the user may create or modify levels.
func (user User) CanManageLevels() bool {
	return user.Role == UserRoleAdmin
}

// Valid reports whether the user contains a supported role and a name.
func (user User) Valid() bool {
	return user.FullName != "" && (user.Role == UserRoleAdmin || user.Role == UserRoleUser)
}

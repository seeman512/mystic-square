package model

import "time"

// Move stores the zero-based row and column of the tile moved by a player.
// It is represented as a JSON pair, for example [2, 1].
type Move [2]int

// Game stores one player's progress on one level.
// The database has a unique constraint on (user_id, level_id), so a user can
// have at most one game for a given level.
type Game struct {
	ID             uint64     `json:"id" gorm:"primaryKey" example:"1"`
	LevelID        uint64     `json:"level_id" gorm:"not null;index" example:"1"`
	UserID         uint64     `json:"user_id" gorm:"not null;index" example:"1"`
	Init           []int      `json:"init" gorm:"type:jsonb;serializer:json;not null"`
	Moves          []Move     `json:"moves" gorm:"type:jsonb;serializer:json;not null;default:'[]'"`
	IsWin          bool       `json:"is_win" gorm:"not null;default:false" example:"false"`
	IsOver         bool       `json:"-" gorm:"not null;default:false"`
	StartedAt      time.Time  `json:"-" gorm:"not null"`
	LastAccessedAt time.Time  `json:"-" gorm:"not null"`
	CompletedAt    *time.Time `json:"-"`
	// Board is the current derived position. It is not persisted; Init and Moves are the source of truth.
	Board []int `json:"board,omitempty" gorm:"-"`

	Level *Level `json:"level,omitempty" gorm:"foreignKey:LevelID"`
	User  *User  `json:"user,omitempty" gorm:"foreignKey:UserID"`
}

package model

import (
	"errors"
	"strings"
	"time"
)

// Difficulty identifies how challenging a level is intended to be.
type Difficulty string

const (
	DifficultyLow    Difficulty = "low"
	DifficultyMedium Difficulty = "medium"
	DifficultyHard   Difficulty = "hard"
)

const (
	unlimitedLimit       = -1
	defaultGridDimension = 4
	legacyTheme          = "classic"
	legacyObjective      = "solve the puzzle"
)

// Level is a playable puzzle configuration.
type Level struct {
	ID    uint64 `json:"id" example:"1"`
	Title string `json:"title" example:"Ancient Temple"`
	// Theme describes the level's setting, such as a temple, forest, or space station.
	Theme string `json:"theme" example:"temple"`
	// Objective describes what the player must accomplish to complete the level.
	Objective   string     `json:"objective" example:"solve the puzzle"`
	Description *string    `json:"description" example:"A temple hidden in the jungle."`
	Difficulty  Difficulty `json:"difficulty" enums:"low,medium,hard" example:"medium"`
	Columns     int        `json:"columns" minimum:"2" maximum:"100" example:"4"`
	Rows        int        `json:"rows" minimum:"2" maximum:"100" example:"4"`
	TimeLimit   *int       `json:"time_limit" minimum:"-1" maximum:"3600" example:"300"`
	MoveLimit   *int       `json:"move_limit" minimum:"-1" maximum:"10000" example:"50"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// EffectiveTimeLimit returns the configured time limit, or -1 when unlimited.
func (level Level) EffectiveTimeLimit() int {
	if level.TimeLimit == nil {
		return unlimitedLimit
	}
	return *level.TimeLimit
}

// EffectiveMoveLimit returns the configured move limit, or -1 when unlimited.
func (level Level) EffectiveMoveLimit() int {
	if level.MoveLimit == nil {
		return unlimitedLimit
	}
	return *level.MoveLimit
}

// LevelInput contains values supplied when creating or replacing a level.
// Pointers let validation distinguish omitted values from valid zero values; dimensions and limits are optional.
type LevelInput struct {
	Title       *string     `json:"title" example:"Ancient Temple"`
	Theme       *string     `json:"theme" example:"temple"`
	Objective   *string     `json:"objective" example:"solve the puzzle"`
	Description *string     `json:"description" example:"A temple hidden in the jungle."`
	Difficulty  *Difficulty `json:"difficulty" enums:"low,medium,hard" example:"medium"`
	Columns     *int        `json:"columns" minimum:"2" maximum:"100" example:"4"`
	Rows        *int        `json:"rows" minimum:"2" maximum:"100" example:"4"`
	TimeLimit   *int        `json:"time_limit" minimum:"-1" maximum:"3600" example:"300"`
	MoveLimit   *int        `json:"move_limit" minimum:"-1" maximum:"10000" example:"50"`
}

// WithDefaults supplies omitted grid dimensions. Legacy requests that include both
// dimensions but omit both new descriptive fields receive stable default values.
func (input LevelInput) WithDefaults() LevelInput {
	legacyRequest := input.Rows != nil && input.Columns != nil && input.Theme == nil && input.Objective == nil
	if legacyRequest {
		theme, objective := legacyTheme, legacyObjective
		input.Theme = &theme
		input.Objective = &objective
	}
	if input.Rows == nil {
		rows := defaultGridDimension
		input.Rows = &rows
	}
	if input.Columns == nil {
		columns := defaultGridDimension
		input.Columns = &columns
	}
	return input
}

// FieldError describes one invalid level field.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// LevelValidationError describes all invalid fields in a level input.
type LevelValidationError struct {
	Fields []FieldError
}

func (e *LevelValidationError) Error() string {
	if e == nil {
		return ""
	}

	messages := make([]string, len(e.Fields))
	for i, field := range e.Fields {
		messages[i] = field.Field + " " + field.Message
	}
	return strings.Join(messages, "; ")
}

// Validate checks the business rules for a level input.
func (input LevelInput) Validate() error {
	var fields []FieldError
	if input.Title == nil {
		fields = append(fields, FieldError{Field: "title", Message: "is required"})
	} else if strings.TrimSpace(*input.Title) == "" {
		fields = append(fields, FieldError{Field: "title", Message: "must not be blank"})
	}

	if input.Theme == nil {
		fields = append(fields, FieldError{Field: "theme", Message: "is required"})
	} else if strings.TrimSpace(*input.Theme) == "" {
		fields = append(fields, FieldError{Field: "theme", Message: "must not be blank"})
	}
	if input.Objective == nil {
		fields = append(fields, FieldError{Field: "objective", Message: "is required"})
	} else if strings.TrimSpace(*input.Objective) == "" {
		fields = append(fields, FieldError{Field: "objective", Message: "must not be blank"})
	}
	if input.Difficulty == nil {
		fields = append(fields, FieldError{Field: "difficulty", Message: "is required"})
	} else if strings.TrimSpace(string(*input.Difficulty)) == "" {
		fields = append(fields, FieldError{Field: "difficulty", Message: "must not be blank"})
	}

	if input.Rows != nil && (*input.Rows < 2 || *input.Rows > 100) {
		fields = append(fields, FieldError{Field: "rows", Message: "must be between 2 and 100"})
	}
	if input.Columns != nil && (*input.Columns < 2 || *input.Columns > 100) {
		fields = append(fields, FieldError{Field: "columns", Message: "must be between 2 and 100"})
	}
	if input.TimeLimit != nil && (*input.TimeLimit < -1 || *input.TimeLimit > 3600) {
		fields = append(fields, FieldError{Field: "time_limit", Message: "must be between -1 and 3600"})
	}
	if input.MoveLimit != nil && (*input.MoveLimit < -1 || *input.MoveLimit > 10000) {
		fields = append(fields, FieldError{Field: "move_limit", Message: "must be between -1 and 10000"})
	}

	if len(fields) == 0 {
		return nil
	}
	return &LevelValidationError{Fields: fields}
}

// IsValidationError reports whether err is a level validation error.
func IsValidationError(err error) bool {
	var validationErr *LevelValidationError
	return errors.As(err, &validationErr)
}

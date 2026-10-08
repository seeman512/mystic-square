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

// Level is a playable puzzle configuration.
type Level struct {
	ID          uint64     `json:"id"`
	Title       string     `json:"title"`
	Description *string    `json:"description"`
	Difficulty  Difficulty `json:"difficulty"`
	Columns     int        `json:"columns"`
	Rows        int        `json:"rows"`
	TimeLimit   int        `json:"time_limit"`
	MoveLimit   int        `json:"move_limit"`
	CreatedAt   time.Time  `json:"created_at"`
}

// LevelInput contains values supplied when creating or replacing a level.
// Pointers on required fields let validation distinguish omitted values from valid zero values.
type LevelInput struct {
	Title       *string     `json:"title"`
	Description *string     `json:"description"`
	Difficulty  *Difficulty `json:"difficulty"`
	Columns     *int        `json:"columns"`
	Rows        *int        `json:"rows"`
	TimeLimit   *int        `json:"time_limit"`
	MoveLimit   *int        `json:"move_limit"`
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

	if input.Difficulty == nil {
		fields = append(fields, FieldError{Field: "difficulty", Message: "is required"})
	} else {
		switch *input.Difficulty {
		case DifficultyLow, DifficultyMedium, DifficultyHard:
		default:
			fields = append(fields, FieldError{
				Field:   "difficulty",
				Message: "must be one of low, medium, or hard",
			})
		}
	}

	if input.Rows == nil {
		fields = append(fields, FieldError{Field: "rows", Message: "is required"})
	} else if *input.Rows < 2 || *input.Rows > 100 {
		fields = append(fields, FieldError{Field: "rows", Message: "must be between 2 and 100"})
	}
	if input.Columns == nil {
		fields = append(fields, FieldError{Field: "columns", Message: "is required"})
	} else if *input.Columns < 2 || *input.Columns > 100 {
		fields = append(fields, FieldError{Field: "columns", Message: "must be between 2 and 100"})
	}
	if input.TimeLimit == nil {
		fields = append(fields, FieldError{Field: "time_limit", Message: "is required"})
	} else if *input.TimeLimit < -1 || *input.TimeLimit > 3600 {
		fields = append(fields, FieldError{Field: "time_limit", Message: "must be between -1 and 3600"})
	}
	if input.MoveLimit == nil {
		fields = append(fields, FieldError{Field: "move_limit", Message: "is required"})
	} else if *input.MoveLimit < -1 || *input.MoveLimit > 10000 {
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

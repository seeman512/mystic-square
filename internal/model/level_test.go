package model

import (
	"errors"
	"strings"
	"testing"
)

func TestLevelInputValidate(t *testing.T) {
	tests := []struct {
		name  string
		input LevelInput
		valid bool
	}{
		{
			name: "valid",
			input: LevelInput{
				Title: pointer("Starter"), Difficulty: pointer(DifficultyLow),
				Rows: pointer(4), Columns: pointer(4), TimeLimit: pointer(-1), MoveLimit: pointer(100),
			},
			valid: true,
		},
		{
			name: "valid boundaries",
			input: LevelInput{
				Title: pointer("Starter"), Difficulty: pointer(DifficultyHard),
				Rows: pointer(2), Columns: pointer(100), TimeLimit: pointer(3600), MoveLimit: pointer(10000),
			},
			valid: true,
		},
		{
			name: "zero limits are present values",
			input: LevelInput{
				Title: pointer("Starter"), Difficulty: pointer(DifficultyLow),
				Rows: pointer(2), Columns: pointer(2), TimeLimit: pointer(0), MoveLimit: pointer(0),
			},
			valid: true,
		},
		{
			name: "blank title",
			input: LevelInput{
				Title: pointer(" "), Difficulty: pointer(DifficultyLow),
				Rows: pointer(4), Columns: pointer(4), TimeLimit: pointer(1), MoveLimit: pointer(1),
			},
		},
		{
			name: "arbitrary nonblank difficulty",
			input: LevelInput{
				Title: pointer("Starter"), Difficulty: pointer(Difficulty("easy")),
				Rows: pointer(4), Columns: pointer(4), TimeLimit: pointer(1), MoveLimit: pointer(1),
			},
			valid: true,
		},
		{
			name: "too few rows",
			input: LevelInput{
				Title: pointer("Starter"), Difficulty: pointer(DifficultyLow),
				Rows: pointer(1), Columns: pointer(2), TimeLimit: pointer(1), MoveLimit: pointer(1),
			},
		},
		{
			name: "too many columns",
			input: LevelInput{
				Title: pointer("Starter"), Difficulty: pointer(DifficultyLow),
				Rows: pointer(2), Columns: pointer(101), TimeLimit: pointer(1), MoveLimit: pointer(1),
			},
		},
		{
			name: "invalid time",
			input: LevelInput{
				Title: pointer("Starter"), Difficulty: pointer(DifficultyLow),
				Rows: pointer(4), Columns: pointer(4), TimeLimit: pointer(-2), MoveLimit: pointer(1),
			},
		},
		{
			name: "time too high",
			input: LevelInput{
				Title: pointer("Starter"), Difficulty: pointer(DifficultyLow),
				Rows: pointer(4), Columns: pointer(4), TimeLimit: pointer(3601), MoveLimit: pointer(1),
			},
		},
		{
			name: "moves too high",
			input: LevelInput{
				Title: pointer("Starter"), Difficulty: pointer(DifficultyLow),
				Rows: pointer(4), Columns: pointer(4), TimeLimit: pointer(1), MoveLimit: pointer(10001),
			},
		},
		{name: "missing required fields", input: LevelInput{}},
	}
	for i := range tests {
		if tests[i].name != "missing required fields" {
			tests[i].input.Theme = pointer("forest")
			tests[i].input.Objective = pointer("find the exit")
		}
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if (err == nil) != tt.valid {
				t.Fatalf("Validate() error = %v, valid = %v", err, tt.valid)
			}
		})
	}
}

func TestLevelInputValidateAllowsOmittedLimits(t *testing.T) {
	input := LevelInput{
		Title: pointer("Starter"), Theme: pointer("forest"), Objective: pointer("find the exit"),
		Difficulty: pointer(DifficultyLow),
	}
	if err := input.Validate(); err != nil {
		t.Fatalf("Validate() with omitted dimensions and limits = %v, want nil", err)
	}
	defaulted := input.WithDefaults()
	if *defaulted.Rows != 4 || *defaulted.Columns != 4 {
		t.Errorf("default dimensions = %dx%d, want 4x4", *defaulted.Columns, *defaulted.Rows)
	}

	level := Level{}
	if got := level.EffectiveTimeLimit(); got != -1 {
		t.Errorf("EffectiveTimeLimit() = %d, want -1 for an omitted limit", got)
	}
	if got := level.EffectiveMoveLimit(); got != -1 {
		t.Errorf("EffectiveMoveLimit() = %d, want -1 for an omitted limit", got)
	}

	timeLimit, moveLimit := 120, 50
	level.TimeLimit = &timeLimit
	level.MoveLimit = &moveLimit
	if got := level.EffectiveTimeLimit(); got != timeLimit {
		t.Errorf("EffectiveTimeLimit() = %d, want %d", got, timeLimit)
	}
	if got := level.EffectiveMoveLimit(); got != moveLimit {
		t.Errorf("EffectiveMoveLimit() = %d, want %d", got, moveLimit)
	}
}

func TestLevelInputValidateReturnsAllErrors(t *testing.T) {
	err := (LevelInput{
		Title:      pointer(" "),
		Difficulty: pointer(Difficulty(" ")),
		TimeLimit:  pointer(-2),
		MoveLimit:  pointer(-2),
	}).Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want all validation errors")
	}

	var validationErr *LevelValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("Validate() error type = %T, want *LevelValidationError", err)
	}

	gotFields := make(map[string]bool, len(validationErr.Fields))
	for _, fieldErr := range validationErr.Fields {
		gotFields[fieldErr.Field] = true
	}
	for _, field := range []string{
		"title",
		"theme",
		"objective",
		"difficulty",
		"time_limit",
		"move_limit",
	} {
		if !gotFields[field] {
			t.Errorf("Validate() error fields = %+v, want field %q", validationErr.Fields, field)
		}
		if !strings.Contains(err.Error(), field) {
			t.Errorf("Validate() error = %q, want field %q", err, field)
		}
	}
}

func pointer[T any](value T) *T {
	return &value
}

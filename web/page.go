package web

import (
	"context"

	"mystic-square/internal/model"
)

// LevelLister lists levels available to the game page.
type LevelLister interface {
	List(context.Context) ([]model.Level, error)
}

// LevelAdminService contains the validated operations used by the web admin
// levels page.
type LevelAdminService interface {
	LevelLister
	Create(context.Context, model.LevelInput) (model.Level, error)
	UpdateTitleDescription(context.Context, uint64, string, *string) (model.Level, error)
}

// UserAdminService contains the validated operations used by the web admin
// users page.
type UserAdminService interface {
	List(context.Context) ([]model.User, error)
	UpdateRole(context.Context, uint64, model.UserRole) (model.User, error)
}

// GamePageService contains the operations required by the authenticated game page.
type GamePageService interface {
	Create(context.Context, uint64, uint64) (model.Game, error)
	ListForUser(context.Context, uint64) ([]model.Game, error)
	GetForUser(context.Context, uint64, uint64) (model.Game, error)
	MoveForUser(context.Context, uint64, uint64, model.Move) (model.Game, error)
}

// PageData is the deliberately small view model exposed to the templates.
// It does not contain authentication tokens, password hashes, or the complete User value.
type PageData struct {
	Title         string
	Date          string
	Authenticated bool
	IsAdmin       bool
	FullName      string
	Email         string
	FullNameInput string
	Errors        []string
	Error         string
	IsLogin       bool
	IsRegister    bool
	IsGame        bool
	IsLevels      bool
	IsUsers       bool
	CSRFToken     string
	AdminLevels   []AdminLevelPage
	AdminUsers    []AdminUserPage
	LevelForm     AdminLevelForm
	AdminNotice   string
	AdminError    string
	Levels        []LevelPage
	Games         []GameSummaryPage
	SelectedGame  *GamePage
	GameNotice    string
	GameError     string
}

// LevelPage is the safe level view exposed to the game template.
type LevelPage struct {
	ID              uint64
	Title           string
	Difficulty      string
	Rows            int
	Columns         int
	MoveLimit       int
	MoveLimitLabel  string
	ExistingGameID  uint64
	HasExistingGame bool
}

// GameSummaryPage is the safe summary view shown in the games panel.
type GameSummaryPage struct {
	ID          uint64
	LevelTitle  string
	Rows        int
	Columns     int
	Status      string
	Moves       int
	Elapsed     string
	Selected    bool
	CanContinue bool
}

// GamePage is the safe current-game view exposed to the game template.
type GamePage struct {
	ID         uint64
	LevelTitle string
	Difficulty string
	Rows       int
	Columns    int
	Moves      int
	MaxMoves   int
	Elapsed    string
	Status     string
	Message    string
	Error      string
	CanMove    bool
	BoardRows  [][]BoardCell
}

// AdminLevelPage is the safe level view exposed to administrator templates.
type AdminLevelPage struct {
	ID          uint64
	CSRFToken   string
	Title       string
	Description string
	Theme       string
	Objective   string
	Difficulty  string
	Rows        int
	Columns     int
	TimeLimit   string
	MoveLimit   string
	CreatedAt   string
	UpdatedAt   string
}

// AdminUserPage is the safe user view exposed to administrator templates.
type AdminUserPage struct {
	ID        uint64
	CSRFToken string
	FullName  string
	Email     string
	Role      string
	CreatedAt string
}

// AdminLevelForm contains create-level values and validation feedback.
type AdminLevelForm struct {
	Title       string
	Theme       string
	Objective   string
	Description string
	Difficulty  string
	Rows        string
	Columns     string
	TimeLimit   string
	MoveLimit   string
	Errors      []string
}

// BoardCell describes one rendered puzzle cell.
type BoardCell struct {
	Value     int
	Row       int
	Column    int
	Empty     bool
	Movable   bool
	AriaLabel string
}

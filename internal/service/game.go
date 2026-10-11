package service

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"mystic-square/internal/model"
	"mystic-square/internal/repository"
)

const shuffleMoves = 1000

var shuffleSeed atomic.Int64

var (
	// ErrGameFinished indicates that no moves can be made after winning.
	ErrGameFinished = errors.New("game is already won")
	// ErrInvalidMove indicates a coordinate that is outside the board or not next to the empty cell.
	ErrInvalidMove = errors.New("invalid move")
)

// InvalidMoveError describes why a requested move cannot be applied.
type InvalidMoveError struct {
	Row    int
	Column int
	Reason string
}

func (e *InvalidMoveError) Error() string {
	if e == nil || e.Reason == "" {
		return ErrInvalidMove.Error()
	}
	return fmt.Sprintf("%s: %s", ErrInvalidMove, e.Reason)
}

func (e *InvalidMoveError) Unwrap() error { return ErrInvalidMove }

// GameService creates games and applies legal sliding-puzzle moves.
type GameService struct {
	games  repository.GameRepository
	levels repository.LevelRepository
	users  repository.UserRepository
	moveMu sync.Mutex
}

// NewGameService creates a game service using the supplied repositories.
func NewGameService(
	gameRepository repository.GameRepository,
	levelRepository repository.LevelRepository,
	userRepository repository.UserRepository,
) *GameService {
	return &GameService{games: gameRepository, levels: levelRepository, users: userRepository}
}

// Create creates a solvable game for a user and level. The initial position is
// produced by applying legal random moves to the solved board, so it is always solvable.
func (s *GameService) Create(ctx context.Context, userID, levelID uint64) (model.Game, error) {
	if err := contextError(ctx); err != nil {
		return model.Game{}, err
	}
	if _, err := s.users.Get(ctx, userID); err != nil {
		return model.Game{}, err
	}
	level, err := s.levels.Get(ctx, levelID)
	if err != nil {
		return model.Game{}, err
	}

	board, err := shuffledBoard(ctx, level.Rows, level.Columns)
	if err != nil {
		return model.Game{}, err
	}
	game := model.Game{
		LevelID: levelID,
		UserID:  userID,
		Init:    board,
		Moves:   []model.Move{},
		Board:   append([]int(nil), board...),
	}
	return s.games.Create(ctx, game)
}

// Get returns a game with its current board derived from Init and Moves.
func (s *GameService) Get(ctx context.Context, id uint64) (model.Game, error) {
	game, err := s.games.Get(ctx, id)
	if err != nil {
		return model.Game{}, err
	}
	return s.withBoard(ctx, game)
}

// List returns games with their current boards derived from Init and Moves.
func (s *GameService) List(ctx context.Context) ([]model.Game, error) {
	games, err := s.games.List(ctx)
	if err != nil {
		return nil, err
	}
	for i := range games {
		games[i], err = s.withBoard(ctx, games[i])
		if err != nil {
			return nil, err
		}
	}
	return games, nil
}

// Move applies one zero-based coordinate move to a game.
func (s *GameService) Move(ctx context.Context, id uint64, move model.Move) (model.Game, error) {
	if err := contextError(ctx); err != nil {
		return model.Game{}, err
	}
	s.moveMu.Lock()
	defer s.moveMu.Unlock()

	game, err := s.games.Get(ctx, id)
	if err != nil {
		return model.Game{}, err
	}
	if game.IsWin {
		return model.Game{}, ErrGameFinished
	}
	level, err := s.levels.Get(ctx, game.LevelID)
	if err != nil {
		return model.Game{}, err
	}
	board, err := boardAfterMoves(game.Init, game.Moves, level.Rows, level.Columns)
	if err != nil {
		return model.Game{}, fmt.Errorf("load game %d board: %w", id, err)
	}
	if err := applyMove(board, move, level.Rows, level.Columns); err != nil {
		return model.Game{}, err
	}
	game.Moves = append(game.Moves, move)
	game.IsWin = solved(board)
	game.Board = board
	return s.games.Update(ctx, game)
}

func (s *GameService) withBoard(ctx context.Context, game model.Game) (model.Game, error) {
	var (
		level model.Level
		err   error
	)
	if game.Level != nil {
		level = *game.Level
	} else {
		level, err = s.levels.Get(ctx, game.LevelID)
		if err != nil {
			return model.Game{}, err
		}
	}
	board, err := boardAfterMoves(game.Init, game.Moves, level.Rows, level.Columns)
	if err != nil {
		return model.Game{}, fmt.Errorf("load game %d board: %w", game.ID, err)
	}
	game.Board = board
	return game, nil
}

func shuffledBoard(ctx context.Context, rows, columns int) ([]int, error) {
	if rows < 2 || columns < 2 {
		return nil, fmt.Errorf("board dimensions must be at least 2")
	}
	board := make([]int, rows*columns)
	for i := range board {
		board[i] = i + 1
	}
	board[len(board)-1] = 0

	seed := time.Now().UnixNano() + shuffleSeed.Add(1)
	random := rand.New(rand.NewSource(seed))
	for i := 0; i < shuffleMoves; i++ {
		if err := contextError(ctx); err != nil {
			return nil, err
		}
		empty := findEmpty(board)
		neighbors := adjacentIndexes(empty, rows, columns)
		if err := applyIndexMove(board, neighbors[random.Intn(len(neighbors))]); err != nil {
			return nil, err
		}
	}
	if solved(board) {
		// Keep a newly created game in progress even if the random walk returns
		// to the solved position (which is possible on small boards).
		empty := findEmpty(board)
		if err := applyIndexMove(board, adjacentIndexes(empty, rows, columns)[0]); err != nil {
			return nil, err
		}
	}
	return board, nil
}

func boardAfterMoves(initial []int, moves []model.Move, rows, columns int) ([]int, error) {
	if rows < 2 || columns < 2 || len(initial) != rows*columns {
		return nil, fmt.Errorf("stored game board has invalid dimensions")
	}
	board := append([]int(nil), initial...)
	for _, move := range moves {
		if err := applyMove(board, move, rows, columns); err != nil {
			return nil, fmt.Errorf("stored game contains invalid move: %w", err)
		}
	}
	return board, nil
}

func applyMove(board []int, move model.Move, rows, columns int) error {
	row, column := move[0], move[1]
	if row < 0 || row >= rows || column < 0 || column >= columns {
		return &InvalidMoveError{Row: row, Column: column, Reason: "coordinates are outside the board"}
	}
	index := row*columns + column
	empty := findEmpty(board)
	if empty < 0 {
		return fmt.Errorf("board has no empty cell")
	}
	if abs(row-empty/columns)+abs(column-empty%columns) != 1 {
		return &InvalidMoveError{Row: row, Column: column, Reason: "tile is not adjacent to the empty cell"}
	}
	board[index], board[empty] = board[empty], board[index]
	return nil
}

func applyIndexMove(board []int, index int) error {
	empty := findEmpty(board)
	if empty < 0 {
		return fmt.Errorf("board has no empty cell")
	}
	if index < 0 || index >= len(board) {
		return fmt.Errorf("shuffle selected an invalid cell")
	}
	board[index], board[empty] = board[empty], board[index]
	return nil
}

func adjacentIndexes(index, rows, columns int) []int {
	row, column := index/columns, index%columns
	neighbors := make([]int, 0, 4)
	if row > 0 {
		neighbors = append(neighbors, index-columns)
	}
	if row+1 < rows {
		neighbors = append(neighbors, index+columns)
	}
	if column > 0 {
		neighbors = append(neighbors, index-1)
	}
	if column+1 < columns {
		neighbors = append(neighbors, index+1)
	}
	return neighbors
}

func findEmpty(board []int) int {
	for index, value := range board {
		if value == 0 {
			return index
		}
	}
	return -1
}

func solved(board []int) bool {
	for index, value := range board {
		want := index + 1
		if index == len(board)-1 {
			want = 0
		}
		if value != want {
			return false
		}
	}
	return true
}

// IsConflict reports whether err violates a repository uniqueness constraint.
func IsConflict(err error) bool {
	return errors.Is(err, repository.ErrConflict)
}

// IsInvalidMove reports whether err describes an illegal puzzle move.
func IsInvalidMove(err error) bool {
	return errors.Is(err, ErrInvalidMove)
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func contextError(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	return ctx.Err()
}

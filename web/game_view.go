package web

import (
	"fmt"
	"net/http"
	"time"

	"mystic-square/internal/model"
	"mystic-square/internal/service"
)

func (h *Handler) gamePageData(r *http.Request, user model.User, selectedID uint64, notice, gameError string) (PageData, error) {
	levels, err := h.levels.List(r.Context())
	if err != nil {
		return PageData{}, fmt.Errorf("list levels for game page: %w", err)
	}
	games, err := h.games.ListForUser(r.Context(), user.ID)
	if err != nil {
		return PageData{}, fmt.Errorf("list user games for game page: %w", err)
	}
	var selected *model.Game
	if selectedID != 0 {
		loaded, loadErr := h.games.GetForUser(r.Context(), user.ID, selectedID)
		if loadErr != nil {
			return PageData{}, loadErr
		}
		selected = &loaded
		games, err = h.games.ListForUser(r.Context(), user.ID)
		if err != nil {
			return PageData{}, fmt.Errorf("refresh user games for game page: %w", err)
		}
	}

	levelByID := make(map[uint64]model.Level, len(levels))
	for _, level := range levels {
		levelByID[level.ID] = level
	}
	gameByLevel := make(map[uint64]model.Game, len(games))
	for _, game := range games {
		gameByLevel[game.LevelID] = game
	}
	data := h.pageData("Game", r, PageData{
		Authenticated: true,
		FullName:      user.FullName,
		IsGame:        true,
		CSRFToken:     csrfTokenFromRequest(r),
		GameNotice:    notice,
		GameError:     gameError,
	})
	data.Levels = make([]LevelPage, 0, len(levels))
	for _, level := range levels {
		limit := service.EffectiveMoveLimit(level)
		view := LevelPage{
			ID:             level.ID,
			Title:          level.Title,
			Difficulty:     string(level.Difficulty),
			Rows:           level.Rows,
			Columns:        level.Columns,
			MoveLimit:      limit,
			MoveLimitLabel: fmt.Sprintf("%d moves", limit),
		}
		if existing, ok := gameByLevel[level.ID]; ok {
			view.HasExistingGame = true
			view.ExistingGameID = existing.ID
		}
		data.Levels = append(data.Levels, view)
	}
	data.Games = make([]GameSummaryPage, 0, len(games))
	for _, game := range games {
		level := levelByID[game.LevelID]
		data.Games = append(data.Games, gameSummaryPage(game, level, selectedID, h.now()))
	}
	if selected != nil {
		level, ok := levelByID[selected.LevelID]
		if !ok {
			return PageData{}, fmt.Errorf("level %d for game %d is missing", selected.LevelID, selected.ID)
		}
		view, viewErr := gamePageView(*selected, level, h.now(), gameError)
		if viewErr != nil {
			return PageData{}, viewErr
		}
		data.SelectedGame = &view
	}
	return data, nil
}

func gameStatus(game model.Game) string {
	if game.IsWin {
		return "Completed"
	}
	if game.IsOver {
		return "Game over"
	}
	return "In progress"
}

func gameSummaryPage(game model.Game, level model.Level, selectedID uint64, now time.Time) GameSummaryPage {
	status := gameStatus(game)
	return GameSummaryPage{
		ID:          game.ID,
		LevelTitle:  level.Title,
		Rows:        level.Rows,
		Columns:     level.Columns,
		Status:      status,
		Moves:       len(game.Moves),
		Elapsed:     elapsedTime(game, now),
		Selected:    game.ID == selectedID,
		CanContinue: status == "In progress",
	}
}

func gamePageView(game model.Game, level model.Level, now time.Time, message string) (GamePage, error) {
	if len(game.Board) != level.Rows*level.Columns {
		return GamePage{}, fmt.Errorf("game %d has an invalid board", game.ID)
	}
	empty := -1
	for index, value := range game.Board {
		if value == 0 {
			empty = index
			break
		}
	}
	if empty < 0 {
		return GamePage{}, fmt.Errorf("game %d has no empty cell", game.ID)
	}
	status := gameStatus(game)
	view := GamePage{
		ID:         game.ID,
		LevelTitle: level.Title,
		Difficulty: string(level.Difficulty),
		Rows:       level.Rows,
		Columns:    level.Columns,
		Moves:      len(game.Moves),
		MaxMoves:   service.EffectiveMoveLimit(level),
		Elapsed:    elapsedTime(game, now),
		Status:     status,
		Message:    message,
		CanMove:    status == "In progress",
		BoardRows:  make([][]BoardCell, level.Rows),
	}
	for row := 0; row < level.Rows; row++ {
		view.BoardRows[row] = make([]BoardCell, level.Columns)
		for column := 0; column < level.Columns; column++ {
			index := row*level.Columns + column
			value := game.Board[index]
			movable := empty >= 0 && absInt(row-empty/level.Columns)+absInt(column-empty%level.Columns) == 1
			view.BoardRows[row][column] = BoardCell{
				Value:     value,
				Row:       row,
				Column:    column,
				Empty:     value == 0,
				Movable:   movable && value != 0 && view.CanMove,
				AriaLabel: boardCellLabel(value, row, column),
			}
		}
	}
	return view, nil
}

func boardCellLabel(value, row, column int) string {
	if value == 0 {
		return fmt.Sprintf("Empty cell, row %d, column %d", row+1, column+1)
	}
	return fmt.Sprintf("Tile %d, row %d, column %d", value, row+1, column+1)
}

func elapsedTime(game model.Game, now time.Time) string {
	start := game.StartedAt
	if start.IsZero() {
		start = now
	}
	end := now
	if game.CompletedAt != nil {
		end = *game.CompletedAt
	}
	if end.Before(start) {
		end = start
	}
	duration := end.Sub(start).Round(time.Second)
	hours := int(duration / time.Hour)
	duration -= time.Duration(hours) * time.Hour
	minutes := int(duration / time.Minute)
	duration -= time.Duration(minutes) * time.Minute
	seconds := int(duration / time.Second)
	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

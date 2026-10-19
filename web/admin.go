package web

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"mystic-square/internal/model"
	"mystic-square/internal/repository"
	"mystic-square/internal/service"
)

func (h *Handler) root(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		h.methodNotAllowed(w, http.MethodGet)
		return
	}

	if _, ok := h.currentUser(r); ok {
		h.redirect(w, r, "/game")
		return
	}
	h.expireSessionIfPresent(w, r)
	h.redirect(w, r, "/login")
}

func (h *Handler) levelsAdmin(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		h.levelCreate(w, r)
		return
	}
	if r.Method != http.MethodGet {
		h.methodNotAllowed(w, http.MethodGet, http.MethodPost)
		return
	}
	user, ok := h.requireAdmin(w, r, "levels")
	if !ok {
		return
	}
	if h.adminLevels == nil {
		h.renderAdminError(w, r, "Levels", user, "level administration is not configured", http.StatusInternalServerError, "levels")
		return
	}

	data, err := h.adminLevelsPageData(r, user, "levels")
	if err != nil {
		h.logger.Error("load admin levels page", "error", err)
		h.renderAdminError(w, r, "Levels", user, "we could not load the levels page", http.StatusInternalServerError, "levels")
		return
	}
	data.CSRFToken = h.ensureCSRFToken(w, r)
	applyAdminCSRF(&data)
	h.renderAdminPage(w, r, "levels", data, http.StatusOK)
}

func (h *Handler) levelCreate(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireAdmin(w, r, "levels")
	if !ok {
		return
	}
	if h.adminLevels == nil {
		h.renderAdminError(w, r, "Levels", user, "level administration is not configured", http.StatusInternalServerError, "levels")
		return
	}

	values, err := parseForm(w, r)
	if err != nil {
		h.renderAdminLevelCreateError(w, r, user, http.StatusUnprocessableEntity, "Please provide valid level values.", values)
		return
	}
	if !h.validCSRF(r) {
		h.renderAdminLevelCreateError(w, r, user, http.StatusForbidden, "Your session form has expired. Please refresh and try again.", values)
		return
	}

	input, inputErrors := levelInputFromValues(values)
	if len(inputErrors) > 0 {
		h.renderAdminLevelCreateError(w, r, user, http.StatusUnprocessableEntity, strings.Join(inputErrors, " "), values)
		return
	}
	if _, err := h.adminLevels.Create(r.Context(), input); err != nil {
		status, message := adminLevelServiceError(err)
		h.renderAdminLevelCreateError(w, r, user, status, message, values)
		return
	}
	h.redirect(w, r, "/levels")
}

func (h *Handler) levelEdit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.methodNotAllowed(w, http.MethodPost)
		return
	}
	id, err := adminIDFromPath(r.URL.Path, "levels")
	if err != nil {
		h.renderError(w, r, "Levels", "invalid level id", http.StatusBadRequest)
		return
	}
	user, ok := h.requireAdmin(w, r, "levels")
	if !ok {
		return
	}
	if h.adminLevels == nil {
		h.renderAdminError(w, r, "Levels", user, "level administration is not configured", http.StatusInternalServerError, "levels")
		return
	}

	values, err := parseForm(w, r)
	if err != nil {
		h.renderAdminLevelActionError(w, r, user, id, http.StatusUnprocessableEntity, "Please provide a valid level title and description.", values)
		return
	}
	if !h.validCSRF(r) {
		h.renderAdminLevelActionError(w, r, user, id, http.StatusForbidden, "Your session form has expired. Please refresh and try again.", values)
		return
	}

	title := strings.TrimSpace(values.Get("title"))
	description := optionalDescription(values.Get("description"))
	if title == "" {
		h.renderAdminLevelActionError(w, r, user, id, http.StatusUnprocessableEntity, "Title must not be blank.", values)
		return
	}
	if _, err := h.adminLevels.UpdateTitleDescription(r.Context(), id, title, description); err != nil {
		status, message := adminLevelServiceError(err)
		h.renderAdminLevelActionError(w, r, user, id, status, message, values)
		return
	}
	h.redirect(w, r, "/levels")
}

func (h *Handler) usersAdmin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.methodNotAllowed(w, http.MethodGet)
		return
	}
	user, ok := h.requireAdmin(w, r, "users")
	if !ok {
		return
	}
	if h.adminUsers == nil {
		h.renderAdminError(w, r, "Users", user, "user administration is not configured", http.StatusInternalServerError, "users")
		return
	}

	data, err := h.adminUsersPageData(r, user)
	if err != nil {
		h.logger.Error("load admin users page", "error", err)
		h.renderAdminError(w, r, "Users", user, "we could not load the users page", http.StatusInternalServerError, "users")
		return
	}
	data.CSRFToken = h.ensureCSRFToken(w, r)
	applyAdminCSRF(&data)
	h.renderAdminPage(w, r, "users", data, http.StatusOK)
}

func (h *Handler) userRole(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.methodNotAllowed(w, http.MethodPost)
		return
	}
	id, err := adminRoleUserID(r.URL.Path)
	if err != nil {
		h.renderError(w, r, "Users", "invalid user id", http.StatusBadRequest)
		return
	}
	user, ok := h.requireAdmin(w, r, "users")
	if !ok {
		return
	}
	if h.adminUsers == nil {
		h.renderAdminError(w, r, "Users", user, "user administration is not configured", http.StatusInternalServerError, "users")
		return
	}

	values, err := parseForm(w, r)
	if err != nil {
		h.renderAdminUserActionError(w, r, user, http.StatusUnprocessableEntity, "Please choose a valid role.")
		return
	}
	if !h.validCSRF(r) {
		h.renderAdminUserActionError(w, r, user, http.StatusForbidden, "Your session form has expired. Please refresh and try again.")
		return
	}

	role := model.UserRole(strings.TrimSpace(values.Get("role")))
	if role != model.UserRoleAdmin && role != model.UserRoleUser {
		h.renderAdminUserActionError(w, r, user, http.StatusUnprocessableEntity, "Role must be admin or user.")
		return
	}
	if _, err := h.adminUsers.UpdateRole(r.Context(), id, role); err != nil {
		status, message := adminUserServiceError(err)
		h.renderAdminUserActionError(w, r, user, status, message)
		return
	}
	h.redirect(w, r, "/users")
}

func (h *Handler) requireAdmin(w http.ResponseWriter, r *http.Request, page string) (model.User, bool) {
	user, ok := h.currentUser(r)
	if !ok {
		h.expireSessionIfPresent(w, r)
		h.redirect(w, r, "/login")
		return model.User{}, false
	}
	if user.Role != model.UserRoleAdmin {
		title := "Levels"
		if page == "users" {
			title = "Users"
		}
		h.renderAdminError(w, r, title, user, "You do not have permission to access this page.", http.StatusForbidden, page)
		return model.User{}, false
	}
	return user, true
}

func (h *Handler) adminLevelsPageData(r *http.Request, user model.User, page string) (PageData, error) {
	levels, err := h.adminLevels.List(r.Context())
	if err != nil {
		return PageData{}, fmt.Errorf("list admin levels: %w", err)
	}
	data := h.adminPageData("Levels", r, user, page)
	data.AdminLevels = make([]AdminLevelPage, 0, len(levels))
	for _, level := range levels {
		data.AdminLevels = append(data.AdminLevels, adminLevelPage(level))
	}
	data.LevelForm = AdminLevelForm{Difficulty: string(model.DifficultyMedium)}
	return data, nil
}

func (h *Handler) adminUsersPageData(r *http.Request, user model.User) (PageData, error) {
	users, err := h.adminUsers.List(r.Context())
	if err != nil {
		return PageData{}, fmt.Errorf("list admin users: %w", err)
	}
	data := h.adminPageData("Users", r, user, "users")
	data.AdminUsers = make([]AdminUserPage, 0, len(users))
	for _, listed := range users {
		data.AdminUsers = append(data.AdminUsers, AdminUserPage{
			ID:        listed.ID,
			FullName:  listed.FullName,
			Email:     listed.Email,
			Role:      string(listed.Role),
			CreatedAt: formatAdminTime(listed.CreatedAt),
		})
	}
	return data, nil
}

func (h *Handler) adminPageData(title string, r *http.Request, user model.User, page string) PageData {
	data := h.pageData(title, r, PageData{
		Authenticated: true,
		IsAdmin:       user.Role == model.UserRoleAdmin,
		FullName:      user.FullName,
	})
	if page == "levels" {
		data.IsLevels = true
	} else if page == "users" {
		data.IsUsers = true
	}
	return data
}

func (h *Handler) renderAdminPage(w http.ResponseWriter, r *http.Request, page string, data PageData, status int) {
	name := "layout"
	if isHTMXRequest(r) {
		name = page + "-page"
	}
	h.renderNamedPage(w, r, page, name, data, status)
}

func (h *Handler) renderAdminError(w http.ResponseWriter, r *http.Request, title string, user model.User, message string, status int, page string) {
	if status == http.StatusForbidden {
		data := h.pageData(title, r, PageData{
			Authenticated: true,
			IsAdmin:       user.Role == model.UserRoleAdmin,
			FullName:      user.FullName,
			Error:         message,
		})
		h.renderBasePage(w, r, data, status)
		return
	}

	data := h.adminPageData(title, r, user, page)
	data.AdminError = message
	h.renderPage(w, r, page, data, status)
}

func (h *Handler) renderAdminLevelCreateError(w http.ResponseWriter, r *http.Request, user model.User, status int, message string, values mapValues) {
	data, err := h.adminLevelsPageData(r, user, "levels")
	if err != nil {
		h.logger.Error("reload admin levels page after create error", "error", err)
		h.renderAdminError(w, r, "Levels", user, "we could not load the levels page", http.StatusInternalServerError, "levels")
		return
	}
	data.CSRFToken = csrfTokenFromRequest(r)
	applyAdminCSRF(&data)
	data.AdminError = message
	data.LevelForm = levelFormFromValues(values)
	h.renderAdminPage(w, r, "levels", data, status)
}

func (h *Handler) renderAdminLevelActionError(w http.ResponseWriter, r *http.Request, user model.User, id uint64, status int, message string, values mapValues) {
	data, err := h.adminLevelsPageData(r, user, "levels")
	if err != nil {
		h.logger.Error("reload admin levels page after action error", "error", err)
		h.renderAdminError(w, r, "Levels", user, "we could not load the levels page", http.StatusInternalServerError, "levels")
		return
	}
	data.CSRFToken = csrfTokenFromRequest(r)
	applyAdminCSRF(&data)
	data.AdminError = message
	data.LevelForm = levelFormFromValues(values)
	for index := range data.AdminLevels {
		if data.AdminLevels[index].ID == id {
			data.AdminLevels[index].Title = strings.TrimSpace(values.Get("title"))
			data.AdminLevels[index].Description = values.Get("description")
		}
	}
	h.renderAdminPage(w, r, "levels", data, status)
}

func (h *Handler) renderAdminUserActionError(w http.ResponseWriter, r *http.Request, user model.User, status int, message string) {
	data, err := h.adminUsersPageData(r, user)
	if err != nil {
		h.logger.Error("reload admin users page after action error", "error", err)
		h.renderAdminError(w, r, "Users", user, "we could not load the users page", http.StatusInternalServerError, "users")
		return
	}
	data.CSRFToken = csrfTokenFromRequest(r)
	applyAdminCSRF(&data)
	data.AdminError = message
	h.renderAdminPage(w, r, "users", data, status)
}

func applyAdminCSRF(data *PageData) {
	for index := range data.AdminLevels {
		data.AdminLevels[index].CSRFToken = data.CSRFToken
	}
	for index := range data.AdminUsers {
		data.AdminUsers[index].CSRFToken = data.CSRFToken
	}
}

func levelFormFromValues(values mapValues) AdminLevelForm {
	form := AdminLevelForm{
		Title:       strings.TrimSpace(values.Get("title")),
		Theme:       strings.TrimSpace(values.Get("theme")),
		Objective:   strings.TrimSpace(values.Get("objective")),
		Description: values.Get("description"),
		Difficulty:  strings.TrimSpace(values.Get("difficulty")),
		Rows:        strings.TrimSpace(values.Get("rows")),
		Columns:     strings.TrimSpace(values.Get("columns")),
		TimeLimit:   strings.TrimSpace(values.Get("time_limit")),
		MoveLimit:   strings.TrimSpace(values.Get("move_limit")),
	}
	return form
}

func levelInputFromValues(values mapValues) (model.LevelInput, []string) {
	form := levelFormFromValues(values)
	input := model.LevelInput{
		Title:       stringPointer(form.Title),
		Theme:       stringPointer(form.Theme),
		Objective:   stringPointer(form.Objective),
		Description: optionalDescription(form.Description),
		Difficulty:  difficultyPointer(form.Difficulty),
	}
	var errorsList []string
	if input.Difficulty == nil || (*input.Difficulty != model.DifficultyLow && *input.Difficulty != model.DifficultyMedium && *input.Difficulty != model.DifficultyHard) {
		errorsList = append(errorsList, "Difficulty must be low, medium, or hard.")
	}
	var err error
	if input.Rows, err = optionalIntPointer(form.Rows, "Rows"); err != nil {
		errorsList = append(errorsList, err.Error())
	}
	if input.Columns, err = optionalIntPointer(form.Columns, "Columns"); err != nil {
		errorsList = append(errorsList, err.Error())
	}
	if input.TimeLimit, err = optionalIntPointer(form.TimeLimit, "Time limit"); err != nil {
		errorsList = append(errorsList, err.Error())
	}
	if input.MoveLimit, err = optionalIntPointer(form.MoveLimit, "Move limit"); err != nil {
		errorsList = append(errorsList, err.Error())
	}
	return input, errorsList
}

func optionalIntPointer(value, label string) (*int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return nil, fmt.Errorf("%s must be an integer.", label)
	}
	return &parsed, nil
}

func optionalDescription(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func stringPointer(value string) *string {
	value = strings.TrimSpace(value)
	return &value
}

func difficultyPointer(value string) *model.Difficulty {
	difficulty := model.Difficulty(strings.TrimSpace(value))
	return &difficulty
}

func adminLevelPage(level model.Level) AdminLevelPage {
	description := ""
	if level.Description != nil {
		description = *level.Description
	}
	return AdminLevelPage{
		ID:          level.ID,
		Title:       level.Title,
		Description: description,
		Theme:       level.Theme,
		Objective:   level.Objective,
		Difficulty:  string(level.Difficulty),
		Rows:        level.Rows,
		Columns:     level.Columns,
		TimeLimit:   formatAdminLimit(level.TimeLimit),
		MoveLimit:   formatAdminLimit(level.MoveLimit),
		CreatedAt:   formatAdminTime(level.CreatedAt),
		UpdatedAt:   formatAdminTime(level.UpdatedAt),
	}
}

func formatAdminLimit(value *int) string {
	if value == nil || *value < 0 {
		return "Unlimited"
	}
	return strconv.Itoa(*value)
}

func formatAdminTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

func adminIDFromPath(path, prefix string) (uint64, error) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 2 || parts[0] != prefix {
		return 0, errors.New("invalid admin route")
	}
	return parsePositiveID(parts[1])
}

func adminRoleUserID(path string) (uint64, error) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 3 || parts[0] != "users" || parts[2] != "role" {
		return 0, errors.New("invalid user role route")
	}
	return parsePositiveID(parts[1])
}

func adminLevelServiceError(err error) (int, string) {
	switch {
	case model.IsValidationError(err):
		return http.StatusUnprocessableEntity, err.Error()
	case errors.Is(err, repository.ErrNotFound):
		return http.StatusNotFound, "level not found"
	default:
		return http.StatusInternalServerError, "we could not update the level"
	}
}

func adminUserServiceError(err error) (int, string) {
	if errors.Is(err, repository.ErrNotFound) || service.IsNotFound(err) {
		return http.StatusNotFound, "user not found"
	}
	return http.StatusInternalServerError, "we could not update the user"
}

package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"mystic-square/internal/model"
)

func TestHandler_RootRedirectsByAuthentication(t *testing.T) {
	h := newTestHandler(t)

	guest := httptest.NewRecorder()
	h.ServeHTTP(guest, httptest.NewRequest(http.MethodGet, "/", nil))
	if guest.Code != http.StatusSeeOther || guest.Header().Get("Location") != "/login" {
		t.Fatalf("guest root redirect = %d %q", guest.Code, guest.Header().Get("Location"))
	}

	htmxGuestRequest := httptest.NewRequest(http.MethodGet, "/", nil)
	htmxGuestRequest.Header.Set("HX-Request", "true")
	htmxGuest := httptest.NewRecorder()
	h.ServeHTTP(htmxGuest, htmxGuestRequest)
	if htmxGuest.Code != http.StatusOK || htmxGuest.Header().Get("HX-Redirect") != "/login" || htmxGuest.Header().Get("Vary") != "HX-Request" {
		t.Fatalf("HTMX guest root redirect = %d, redirect=%q, vary=%q", htmxGuest.Code, htmxGuest.Header().Get("HX-Redirect"), htmxGuest.Header().Get("Vary"))
	}

	register := httptest.NewRecorder()
	h.ServeHTTP(register, formRequest(http.MethodPost, "/register", url.Values{
		"email":     {"root@example.test"},
		"full_name": {"Root Player"},
		"password":  {"correct-horse-42"},
	}))
	if register.Code != http.StatusSeeOther {
		t.Fatalf("registration status = %d", register.Code)
	}
	session := register.Result().Cookies()[0]
	if _, err := h.users.UpdateRole(t.Context(), 1, model.UserRoleAdmin); err != nil {
		t.Fatal(err)
	}

	authenticated := httptest.NewRequest(http.MethodGet, "/", nil)
	authenticated.AddCookie(session)
	response := httptest.NewRecorder()
	h.ServeHTTP(response, authenticated)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/game" {
		t.Fatalf("authenticated root redirect = %d %q", response.Code, response.Header().Get("Location"))
	}
}

func TestHandler_AdminPagesAndMutations(t *testing.T) {
	h := newTestHandler(t)
	adminSession := registerAdminForWebTest(t, h, "admin@example.test", "Admin User")

	level := createAdminTestLevel(t, h)
	levelsRequest := httptest.NewRequest(http.MethodGet, "/levels", nil)
	levelsRequest.AddCookie(adminSession)
	levelsResponse := httptest.NewRecorder()
	h.ServeHTTP(levelsResponse, levelsRequest)
	if levelsResponse.Code != http.StatusOK {
		t.Fatalf("levels page status = %d: %s", levelsResponse.Code, levelsResponse.Body.String())
	}
	levelsBody := levelsResponse.Body.String()
	if !strings.Contains(levelsBody, "Levels") || !strings.Contains(levelsBody, "/users") || !strings.Contains(levelsBody, "Ancient &amp; Temple") {
		t.Fatalf("levels page is missing admin content or escaping: %s", levelsBody)
	}
	if !strings.Contains(levelsBody, "admin-levels-table") || !strings.Contains(levelsBody, "table-responsive") || !strings.Contains(levelsBody, "col-lg-3") {
		t.Fatalf("levels page does not use the compact table layout: %s", levelsBody)
	}
	if !strings.Contains(levelsBody, "flex-row flex-wrap") {
		t.Fatalf("admin navigation is not horizontal: %s", levelsBody)
	}
	csrf := cookieByName(levelsResponse.Result().Cookies(), CSRFTokenCookieName)
	if csrf == nil {
		t.Fatal("levels page did not set a CSRF cookie")
	}

	update := httptest.NewRecorder()
	updateRequest := formRequest(http.MethodPost, "/levels/"+uintString(level.ID), url.Values{
		"csrf_token":  {csrf.Value},
		"title":       {"Updated title"},
		"description": {"Updated description"},
	})
	updateRequest.AddCookie(adminSession)
	updateRequest.AddCookie(csrf)
	h.ServeHTTP(update, updateRequest)
	if update.Code != http.StatusSeeOther || update.Header().Get("Location") != "/levels" {
		t.Fatalf("level update = %d %q", update.Code, update.Header().Get("Location"))
	}
	updated, err := h.adminLevels.(interface {
		Get(context.Context, uint64) (model.Level, error)
	}).Get(t.Context(), level.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Title != "Updated title" || updated.Description == nil || *updated.Description != "Updated description" {
		t.Fatalf("updated editable fields = %+v", updated)
	}
	if updated.Theme != level.Theme || updated.Objective != level.Objective || updated.Rows != level.Rows || updated.Columns != level.Columns {
		t.Fatalf("immutable fields changed = %+v", updated)
	}

	create := httptest.NewRecorder()
	createRequest := formRequest(http.MethodPost, "/levels", url.Values{
		"csrf_token": {csrf.Value},
		"title":      {"Created level"},
		"theme":      {"forest"},
		"objective":  {"find the exit"},
		"difficulty": {"hard"},
	})
	createRequest.AddCookie(adminSession)
	createRequest.AddCookie(csrf)
	h.ServeHTTP(create, createRequest)
	if create.Code != http.StatusSeeOther || create.Header().Get("Location") != "/levels" {
		t.Fatalf("level create = %d %q", create.Code, create.Header().Get("Location"))
	}

	regularSession := registerWebUserForTest(t, h, "regular@example.test", "Regular User")
	users := httptest.NewRecorder()
	usersRequest := httptest.NewRequest(http.MethodGet, "/users", nil)
	usersRequest.AddCookie(adminSession)
	h.ServeHTTP(users, usersRequest)
	if users.Code != http.StatusOK || !strings.Contains(users.Body.String(), "regular@example.test") {
		t.Fatalf("users page = %d: %s", users.Code, users.Body.String())
	}
	userCSRF := cookieByName(users.Result().Cookies(), CSRFTokenCookieName)
	if userCSRF == nil {
		t.Fatal("users page did not set a CSRF cookie")
	}

	forbidden := httptest.NewRecorder()
	forbiddenRequest := httptest.NewRequest(http.MethodGet, "/levels", nil)
	forbiddenRequest.AddCookie(regularSession)
	h.ServeHTTP(forbidden, forbiddenRequest)
	if forbidden.Code != http.StatusForbidden || !strings.Contains(forbidden.Body.String(), "permission") {
		t.Fatalf("regular user levels access = %d: %s", forbidden.Code, forbidden.Body.String())
	}
	if strings.Contains(forbidden.Body.String(), "href=\"/users\"") {
		t.Fatal("regular-user forbidden page exposed the admin users link")
	}

	roleUpdate := httptest.NewRecorder()
	roleRequest := formRequest(http.MethodPost, "/users/2/role", url.Values{
		"csrf_token": {userCSRF.Value},
		"role":       {"admin"},
	})
	roleRequest.AddCookie(adminSession)
	roleRequest.AddCookie(userCSRF)
	h.ServeHTTP(roleUpdate, roleRequest)
	if roleUpdate.Code != http.StatusSeeOther || roleUpdate.Header().Get("Location") != "/users" {
		t.Fatalf("role update = %d %q", roleUpdate.Code, roleUpdate.Header().Get("Location"))
	}
	changed, err := h.users.Get(t.Context(), 2)
	if err != nil {
		t.Fatal(err)
	}
	if changed.Role != model.UserRoleAdmin {
		t.Fatalf("role = %q, want admin", changed.Role)
	}
}

func registerAdminForWebTest(t *testing.T, h *Handler, email, fullName string) *http.Cookie {
	t.Helper()
	session := registerWebUserForTest(t, h, email, fullName)
	if _, err := h.users.UpdateRole(t.Context(), 1, model.UserRoleAdmin); err != nil {
		t.Fatal(err)
	}
	return session
}

func registerWebUserForTest(t *testing.T, h *Handler, email, fullName string) *http.Cookie {
	t.Helper()
	response := httptest.NewRecorder()
	h.ServeHTTP(response, formRequest(http.MethodPost, "/register", url.Values{
		"email":     {email},
		"full_name": {fullName},
		"password":  {"correct-horse-42"},
	}))
	if response.Code != http.StatusSeeOther {
		t.Fatalf("registration status = %d: %s", response.Code, response.Body.String())
	}
	cookie := cookieByName(response.Result().Cookies(), SessionCookieName)
	if cookie == nil {
		t.Fatal("registration did not set a session cookie")
	}
	return cookie
}

func createAdminTestLevel(t *testing.T, h *Handler) model.Level {
	t.Helper()
	title := "Ancient & Temple"
	theme := "temple"
	objective := "solve the puzzle"
	difficulty := model.DifficultyMedium
	level, err := h.adminLevels.Create(t.Context(), model.LevelInput{
		Title: &title, Theme: &theme, Objective: &objective, Difficulty: &difficulty,
	})
	if err != nil {
		t.Fatal(err)
	}
	return level
}

func cookieByName(cookies []*http.Cookie, name string) *http.Cookie {
	for _, cookie := range cookies {
		if cookie.Name == name {
			return cookie
		}
	}
	return nil
}

func uintString(value uint64) string {
	return strconv.FormatUint(value, 10)
}

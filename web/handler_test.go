package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"mystic-square/internal/auth"
	"mystic-square/internal/repository"
	"mystic-square/internal/service"
)

const testSecret = "web-test-secret-that-is-at-least-32-bytes-long"

func newTestHandler(t *testing.T) *Handler {
	t.Helper()
	users := repository.NewInMemoryUserRepository()
	levels := repository.NewInMemoryLevelRepository()
	games := repository.NewInMemoryGameRepository()
	authService := auth.NewService(users, testSecret)
	gameService := service.NewGameService(games, levels, users)
	h, err := NewHandler(authService, users, Config{
		Levels:    levels,
		Games:     gameService,
		JWTSecret: testSecret,
		Now: func() time.Time {
			return time.Date(2026, time.October, 5, 12, 0, 0, 0, time.UTC)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func formRequest(method, path string, values url.Values) *http.Request {
	request := httptest.NewRequest(method, path, strings.NewReader(values.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return request
}

func TestHandler_RendersPagesAndLocalAssets(t *testing.T) {
	h := newTestHandler(t)
	for _, test := range []struct {
		name string
		path string
		want string
	}{
		{name: "login", path: "/login", want: "Login"},
		{name: "register", path: "/register", want: "Create an account"},
		{name: "logout", path: "/logout", want: "Goodbye!"},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			h.ServeHTTP(response, httptest.NewRequest(http.MethodGet, test.path, nil))
			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", response.Code)
			}
			body := response.Body.String()
			if !strings.Contains(body, test.want) || !strings.Contains(body, "Mystic Square") {
				t.Fatalf("page does not contain expected title and brand: %s", body)
			}
			if test.path == "/login" || test.path == "/register" {
				if !strings.Contains(body, "/static/js/app.js") || !strings.Contains(body, "hx-ext=\"json-enc\"") {
					t.Fatalf("%s form is not configured for JSON submission: %s", test.path, body)
				}
			}
			if !strings.Contains(body, "2026-10-05") {
				t.Fatal("page does not contain the server-provided footer date")
			}
		})
	}

	response := httptest.NewRecorder()
	h.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/static/js/htmx.min.js", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Header().Get("Content-Type"), "javascript") {
		t.Fatalf("HTMX asset response = %d %q", response.Code, response.Header().Get("Content-Type"))
	}
}

func TestHandler_JSONAuthPayloads(t *testing.T) {
	h := newTestHandler(t)

	register := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(`{"email":"json@example.test","full_name":"JSON Player","password":"correct-horse-42"}`))
	register.Header.Set("Content-Type", "application/json")
	registerResponse := httptest.NewRecorder()
	h.ServeHTTP(registerResponse, register)
	if registerResponse.Code != http.StatusSeeOther || registerResponse.Header().Get("Location") != "/game" {
		t.Fatalf("JSON registration response = %d %q", registerResponse.Code, registerResponse.Header().Get("Location"))
	}
	if len(registerResponse.Result().Cookies()) != 1 {
		t.Fatal("JSON registration did not set a session cookie")
	}

	login := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(`{"email":"json@example.test","password":"correct-horse-42"}`))
	login.Header.Set("Content-Type", "application/json; charset=utf-8")
	loginResponse := httptest.NewRecorder()
	h.ServeHTTP(loginResponse, login)
	if loginResponse.Code != http.StatusSeeOther || loginResponse.Header().Get("Location") != "/game" {
		t.Fatalf("JSON login response = %d %q", loginResponse.Code, loginResponse.Header().Get("Location"))
	}

	unknownField := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(`{"email":"json@example.test","password":"correct-horse-42","role":"admin"}`))
	unknownField.Header.Set("Content-Type", "application/json")
	unknownResponse := httptest.NewRecorder()
	h.ServeHTTP(unknownResponse, unknownField)
	if unknownResponse.Code != http.StatusUnprocessableEntity {
		t.Fatalf("JSON login with an unknown field = %d, want 422", unknownResponse.Code)
	}
}

func TestHandler_RegistrationAndGameEscapeUserName(t *testing.T) {
	h := newTestHandler(t)
	register := httptest.NewRecorder()
	h.ServeHTTP(register, formRequest(http.MethodPost, "/register", url.Values{
		"email":     {"player@example.test"},
		"full_name": {"<b>Ada</b>"},
		"password":  {"correct-horse-42"},
	}))
	if register.Code != http.StatusSeeOther || register.Header().Get("Location") != "/game" {
		t.Fatalf("registration response = %d %q", register.Code, register.Header().Get("Location"))
	}
	cookie := register.Result().Cookies()[0]
	if !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode || cookie.Path != "/" {
		t.Fatalf("session cookie has unsafe attributes: %#v", cookie)
	}

	gameRequest := httptest.NewRequest(http.MethodGet, "/game", nil)
	gameRequest.AddCookie(cookie)
	game := httptest.NewRecorder()
	h.ServeHTTP(game, gameRequest)
	if game.Code != http.StatusOK {
		t.Fatalf("game status = %d, want 200", game.Code)
	}
	body := game.Body.String()
	if strings.Contains(body, "<b>Ada</b>") {
		t.Fatalf("game page rendered the user name as HTML: %s", body)
	}
	if strings.Contains(body, "correct-horse-42") || strings.Contains(body, cookie.Value) {
		t.Fatal("game page exposed a password or JWT")
	}
}

func TestHandler_HTMXValidationAndRedirects(t *testing.T) {
	h := newTestHandler(t)
	invalid := formRequest(http.MethodPost, "/login", url.Values{
		"email":    {"missing@example.test"},
		"password": {"wrong-password"},
	})
	invalid.Header.Set("HX-Request", "true")
	invalidResponse := httptest.NewRecorder()
	h.ServeHTTP(invalidResponse, invalid)
	if invalidResponse.Code != http.StatusUnprocessableEntity {
		t.Fatalf("HTMX invalid login status = %d, want 422", invalidResponse.Code)
	}
	if !strings.Contains(invalidResponse.Body.String(), "Invalid email or password") || strings.Contains(invalidResponse.Body.String(), "<!doctype") {
		t.Fatal("HTMX validation did not return the visible form fragment")
	}
	if invalidResponse.Header().Get("Vary") != "HX-Request" {
		t.Fatalf("Vary = %q, want HX-Request", invalidResponse.Header().Get("Vary"))
	}

	register := formRequest(http.MethodPost, "/register", url.Values{
		"email":     {"player@example.test"},
		"full_name": {"Player"},
		"password":  {"correct-horse-42"},
	})
	register.Header.Set("HX-Request", "true")
	response := httptest.NewRecorder()
	h.ServeHTTP(response, register)
	if response.Code < 200 || response.Code >= 300 || response.Header().Get("HX-Redirect") != "/game" {
		t.Fatalf("HTMX registration redirect = %d %q", response.Code, response.Header().Get("HX-Redirect"))
	}
}

func TestHandler_LogoutExpiresSession(t *testing.T) {
	h := newTestHandler(t)
	request := httptest.NewRequest(http.MethodPost, "/logout", nil)
	response := httptest.NewRecorder()
	h.ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/logout" {
		t.Fatalf("logout response = %d %q", response.Code, response.Header().Get("Location"))
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 1 || cookies[0].MaxAge != -1 || !cookies[0].Expires.Before(time.Now()) {
		t.Fatalf("logout did not expire the cookie: %#v", cookies)
	}
}

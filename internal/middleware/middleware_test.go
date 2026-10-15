package middleware

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRecoveryReturnsCommonJSONError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(Recovery(slog.New(slog.NewTextHandler(httptest.NewRecorder(), nil))))
	router.GET("/panic", func(*gin.Context) { panic("test panic") })

	request := httptest.NewRequest(http.MethodGet, "/panic", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", response.Code)
	}
	if !strings.Contains(response.Body.String(), `"internal_server_error"`) {
		t.Fatalf("response = %s, want common internal error", response.Body)
	}
}

func TestLoggingDoesNotRecordRequestBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var logs bytes.Buffer
	router := gin.New()
	router.Use(Logging(slog.New(slog.NewTextHandler(&logs, nil))))
	router.POST("/login", func(c *gin.Context) { c.Status(http.StatusOK) })

	request := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(`{"email":"a@example.test","password":"secret-value"}`))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if strings.Contains(logs.String(), "secret-value") || strings.Contains(logs.String(), "password") {
		t.Fatalf("request logger exposed credential data: %s", logs.String())
	}
}

func TestRequireRoleAbortsUnauthorizedRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, test := range []struct {
		name   string
		role   string
		status int
	}{
		{name: "missing identity", status: http.StatusUnauthorized},
		{name: "wrong role", role: "user", status: http.StatusForbidden},
		{name: "allowed role", role: "admin", status: http.StatusNoContent},
	} {
		t.Run(test.name, func(t *testing.T) {
			router := gin.New()
			called := false
			router.GET("/admin", func(c *gin.Context) {
				if test.role != "" {
					c.Set("role", test.role)
				}
				c.Next()
			}, RequireRole("admin"), func(c *gin.Context) {
				called = true
				c.Status(http.StatusNoContent)
			})
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/admin", nil))
			if response.Code != test.status {
				t.Fatalf("status = %d, want %d", response.Code, test.status)
			}
			if called != (test.status == http.StatusNoContent) {
				t.Fatalf("protected handler called = %t for status %d", called, test.status)
			}
		})
	}
}

func TestCORSPreflight(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(CORS())
	router.POST("/levels", func(c *gin.Context) { c.Status(http.StatusCreated) })

	request := httptest.NewRequest(http.MethodOptions, "/levels", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", response.Code)
	}
	if response.Header().Get("Access-Control-Allow-Origin") == "" {
		t.Fatal("missing Access-Control-Allow-Origin")
	}
	if !strings.Contains(response.Header().Get("Access-Control-Allow-Methods"), "POST") {
		t.Fatal("CORS methods do not include POST")
	}
}

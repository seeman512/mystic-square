package middleware

import (
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

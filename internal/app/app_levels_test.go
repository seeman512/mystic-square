package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewRouterServesLevels(t *testing.T) {
	router := NewRouter()
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/levels",
		strings.NewReader(`{"title":"Starter","difficulty":"medium","columns":4,"rows":4,"time_limit":-1,"move_limit":100}`),
	)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", response.Code, response.Body)
	}

	request = httptest.NewRequest(http.MethodGet, "/api/v1/levels", nil)
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"title":"Starter"`) {
		t.Fatalf("list response = %d %s", response.Code, response.Body)
	}
}

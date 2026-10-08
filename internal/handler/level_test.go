package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"mystic-square/internal/repository"
	"mystic-square/internal/service"
)

func TestLevelHandlerCreateValidationDetails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name           string
		body           string
		expectedFields map[string]string
	}{
		{
			name: "required fields",
			body: "{}",
			expectedFields: map[string]string{
				"title":      "is required",
				"theme":      "is required",
				"objective":  "is required",
				"difficulty": "is required",
			},
		},
		{
			name: "invalid fields",
			body: `{"title":"Starter","difficulty":" ","columns":101,"rows":1,"time_limit":3601,"move_limit":10001}`,
			expectedFields: map[string]string{
				"difficulty": "must not be blank",
				"columns":    "must be between 2 and 100",
				"rows":       "must be between 2 and 100",
				"time_limit": "must be between -1 and 3600",
				"move_limit": "must be between -1 and 10000",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			h := NewLevelHandler(service.NewLevelService(repository.NewInMemoryLevelRepository()))
			h.RegisterRoutes(router.Group("/api/v1"))

			request := httptest.NewRequest(http.MethodPost, "/api/v1/levels", strings.NewReader(tt.body))
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)

			if response.Code != http.StatusUnprocessableEntity {
				t.Fatalf("create status = %d, body = %s", response.Code, response.Body)
			}
			var body struct {
				Error struct {
					Code    string `json:"code"`
					Message string `json:"message"`
					Fields  []struct {
						Field   string `json:"field"`
						Message string `json:"message"`
					} `json:"fields"`
				} `json:"error"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatalf("unmarshal validation response: %v", err)
			}
			if body.Error.Code != "validation_error" {
				t.Errorf("validation error code = %q, want %q", body.Error.Code, "validation_error")
			}
			if body.Error.Message != "invalid level payload" {
				t.Errorf("validation error message = %q, want %q", body.Error.Message, "invalid level payload")
			}
			gotFields := make(map[string]string, len(body.Error.Fields))
			for _, field := range body.Error.Fields {
				gotFields[field.Field] = field.Message
			}
			if len(gotFields) != len(tt.expectedFields) {
				t.Errorf("validation fields = %v, want %v", gotFields, tt.expectedFields)
			}
			for field, message := range tt.expectedFields {
				if gotFields[field] != message {
					t.Errorf("validation field %q = %q, want %q", field, gotFields[field], message)
				}
			}
		})
	}
}

func TestLevelHandlerCreateDefaultsOmittedGridAndLimits(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	h := NewLevelHandler(service.NewLevelService(repository.NewInMemoryLevelRepository()))
	h.RegisterRoutes(router.Group("/api/v1"))

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/levels",
		strings.NewReader(`{"title":"Starter","theme":"forest","objective":"find the exit","difficulty":"low"}`),
	)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", response.Code, response.Body)
	}

	var level map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &level); err != nil {
		t.Fatalf("unmarshal created level: %v", err)
	}
	for _, field := range []string{"time_limit", "move_limit"} {
		if value, exists := level[field]; !exists || value != nil {
			t.Errorf("omitted %s = %v (present: %t), want null", field, value, exists)
		}
	}
	for _, field := range []string{"rows", "columns"} {
		if value, ok := level[field].(float64); !ok || value != 4 {
			t.Errorf("default %s = %v, want 4", field, level[field])
		}
	}
}

func TestLevelHandlerCreateAndGet(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	h := NewLevelHandler(service.NewLevelService(repository.NewInMemoryLevelRepository()))
	h.RegisterRoutes(router.Group("/api/v1"))

	body := `{"title":"Starter","difficulty":"low","columns":4,"rows":4,"time_limit":-1,"move_limit":100}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/levels", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", response.Code, response.Body)
	}

	request = httptest.NewRequest(http.MethodGet, "/api/v1/levels/1", nil)
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("get status = %d, body = %s", response.Code, response.Body)
	}
}

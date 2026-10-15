package app_test

import (
	"net/http"
	"strconv"
	"testing"
	"time"

	"mystic-square/internal/auth"
	"mystic-square/internal/model"
)

func TestAuthenticationAndRoleAuthorization(t *testing.T) {
	e := newEnv(t)
	registeredResponse := e.doJSON(http.MethodPost, "/api/v1/auth/register", map[string]any{
		"email": "reader@example.test", "full_name": "Mystic Player", "password": "correct-horse-42",
	})
	e.expect(registeredResponse, http.StatusCreated)
	registered := registeredResponse.obj(t)
	registeredUserID := idOf(t, registered["user"].(map[string]any))
	userToken := registered["token"].(string)

	expiredToken, err := auth.SignToken(testJWTSecret, model.User{
		ID: 25, Role: model.UserRoleUser,
	}, time.Now().Add(-time.Minute))
	if err != nil {
		t.Fatal(err)
	}

	rolePath := "/api/v1/users/" + strconv.Itoa(registeredUserID) + "/role"
	levelBody := `{"title":"A level","theme":"classic","objective":"solve","difficulty":"medium"}`
	roleBody := `{"role":"admin"}`

	tests := []struct {
		name           string
		method         string
		path           string
		body           string
		token          string
		expectedStatus int
		expectedCode   string
		expectedRole   string
	}{
		{
			name:           "missing token",
			method:         http.MethodGet,
			path:           "/api/v1/games",
			expectedStatus: http.StatusUnauthorized,
			expectedCode:   "unauthorized",
		},
		{
			name:           "expired token",
			method:         http.MethodGet,
			path:           "/api/v1/games",
			token:          expiredToken,
			expectedStatus: http.StatusUnauthorized,
			expectedCode:   "unauthorized",
		},
		{
			name:           "user cannot create levels",
			method:         http.MethodPost,
			path:           "/api/v1/levels",
			body:           levelBody,
			token:          userToken,
			expectedStatus: http.StatusForbidden,
			expectedCode:   "forbidden",
		},
		{
			name:           "admin can create levels",
			method:         http.MethodPost,
			path:           "/api/v1/levels",
			body:           levelBody,
			token:          e.adminToken,
			expectedStatus: http.StatusCreated,
		},
		{
			name:           "user cannot update roles",
			method:         http.MethodPut,
			path:           rolePath,
			body:           roleBody,
			token:          userToken,
			expectedStatus: http.StatusForbidden,
			expectedCode:   "forbidden",
		},
		{
			name:           "admin cannot set an invalid role",
			method:         http.MethodPut,
			path:           rolePath,
			body:           `{"role":"owner"}`,
			token:          e.adminToken,
			expectedStatus: http.StatusUnprocessableEntity,
			expectedCode:   "validation_error",
		},
		{
			name:           "admin can update roles",
			method:         http.MethodPut,
			path:           rolePath,
			body:           roleBody,
			token:          e.adminToken,
			expectedStatus: http.StatusOK,
			expectedRole:   string(model.UserRoleAdmin),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := e.doWithToken(tt.method, tt.path, tt.body, tt.token)
			if tt.expectedCode != "" {
				e.wantError(response, tt.expectedStatus, tt.expectedCode)
			} else {
				e.expect(response, tt.expectedStatus)
			}

			if tt.expectedRole != "" {
				if got := response.obj(t)["role"]; got != tt.expectedRole {
					t.Fatalf("updated role = %v, want %s", got, tt.expectedRole)
				}
			}
		})
	}
}

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
	for _, test := range []struct {
		name   string
		token  string
		status int
	}{
		{name: "missing token", status: http.StatusUnauthorized},
		{name: "expired token", token: expiredToken, status: http.StatusUnauthorized},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := e.doWithToken(http.MethodGet, "/api/v1/games", "", test.token)
			e.expect(response, test.status)
		})
	}

	for _, test := range []struct {
		name   string
		token  string
		status int
	}{
		{name: "user cannot create levels", token: userToken, status: http.StatusForbidden},
		{name: "admin can create levels", token: e.adminToken, status: http.StatusCreated},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := e.doWithToken(http.MethodPost, "/api/v1/levels", `{"title":"A level","theme":"classic","objective":"solve","difficulty":"medium"}`, test.token)
			e.expect(response, test.status)
		})
	}

	rolePath := "/api/v1/users/" + strconv.Itoa(registeredUserID) + "/role"
	deniedRoleUpdate := e.doWithToken(http.MethodPut, rolePath, `{"role":"admin"}`, userToken)
	e.expect(deniedRoleUpdate, http.StatusForbidden)
	invalidRole := e.doWithToken(http.MethodPut, rolePath, `{"role":"owner"}`, e.adminToken)
	e.expect(invalidRole, http.StatusUnprocessableEntity)
	adminRoleUpdate := e.doWithToken(http.MethodPut, rolePath, `{"role":"admin"}`, e.adminToken)
	e.expect(adminRoleUpdate, http.StatusOK)
	if got := adminRoleUpdate.obj(t)["role"]; got != string(model.UserRoleAdmin) {
		t.Fatalf("updated role = %v, want admin", got)
	}
}

package app_test

import (
	"net/http"
	"strings"
	"testing"

	"mystic-square/internal/model"
)

func TestAuthenticationContracts(t *testing.T) {
	e := newEnv(t)
	registerBody := map[string]any{
		"email": "reader@example.test", "full_name": "Mystic Player", "password": "correct-horse-42",
	}
	registeredResponse := e.doJSON(http.MethodPost, "/api/v1/auth/register", registerBody)
	e.expect(registeredResponse, http.StatusCreated)
	registered := registeredResponse.obj(t)
	registeredUser := registered["user"].(map[string]any)
	if registeredUser["role"] != string(model.UserRoleUser) {
		t.Fatalf("registered role = %v, want user", registeredUser["role"])
	}
	if _, exposed := registeredUser["password_hash"]; exposed {
		t.Fatal("registration response exposed password_hash")
	}

	duplicate := e.doJSON(http.MethodPost, "/api/v1/auth/register", registerBody)
	e.expect(duplicate, http.StatusUnprocessableEntity)

	login := e.doJSON(http.MethodPost, "/api/v1/auth/login", map[string]any{
		"email": "reader@example.test", "password": "correct-horse-42",
	})
	e.expect(login, http.StatusOK)
	if login.obj(t)["token"] == "" {
		t.Fatal("successful login did not return a token")
	}

	wrongPassword := e.doJSON(http.MethodPost, "/api/v1/auth/login", map[string]any{
		"email": "reader@example.test", "password": "wrong-password",
	})
	unknownEmail := e.doJSON(http.MethodPost, "/api/v1/auth/login", map[string]any{
		"email": "missing@example.test", "password": "wrong-password",
	})
	e.expect(wrongPassword, http.StatusUnauthorized)
	e.expect(unknownEmail, http.StatusUnauthorized)
	if wrongPassword.text() != unknownEmail.text() || wrongPassword.text() != `{"error":{"code":"invalid_credentials","message":"invalid credentials"}}` {
		t.Fatalf("login failures differ or are not generic: wrong=%s unknown=%s", wrongPassword.text(), unknownEmail.text())
	}
}

func TestAuthPayloadErrorsDescribeRequirements(t *testing.T) {
	e := newEnv(t)

	tests := []struct {
		name string
		path string
		want []string
	}{
		{
			name: "registration",
			path: "/api/v1/auth/register",
			want: []string{"email", "full_name", "password", "1-200", "8-72"},
		},
		{
			name: "login",
			path: "/api/v1/auth/login",
			want: []string{"email", "password", "non-empty"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := e.do(http.MethodPost, test.path, `{}`)
			e.expect(response, http.StatusUnprocessableEntity)
			message := response.obj(t)["error"].(map[string]any)["message"].(string)
			for _, fragment := range test.want {
				if !strings.Contains(message, fragment) {
					t.Errorf("validation message %q does not contain %q", message, fragment)
				}
			}
		})
	}
}

func TestRegistrationCannotSetRole(t *testing.T) {
	e := newEnv(t)
	response := e.doJSON(http.MethodPost, "/api/v1/auth/register", map[string]any{
		"email": "attacker@example.test", "full_name": "Attacker", "password": "correct-horse-42", "role": "admin",
	})
	e.expect(response, http.StatusUnprocessableEntity)
}

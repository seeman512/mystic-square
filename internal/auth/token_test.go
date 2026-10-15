package auth

import (
	"strings"
	"testing"
	"time"

	"mystic-square/internal/model"
)

func TestSignAndParseToken(t *testing.T) {
	const secret = "unit-test-secret-that-is-at-least-32-bytes-long"
	now := time.Now().UTC()
	user := model.User{ID: 7, Role: model.UserRoleAdmin}
	token, err := SignToken(secret, user, now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	claims, err := ParseToken(secret, token, now)
	if err != nil {
		t.Fatal(err)
	}
	if claims.UserID != user.ID || claims.Role != user.Role || claims.Expiry != now.Add(time.Hour).Unix() {
		t.Fatalf("claims = %+v, want user_id=%d role=%s exp=%d", claims, user.ID, user.Role, now.Add(time.Hour).Unix())
	}
}

func TestParseTokenRejectsTamperedAndExpiredTokens(t *testing.T) {
	const secret = "unit-test-secret-that-is-at-least-32-bytes-long"
	now := time.Now().UTC()
	user := model.User{ID: 7, Role: model.UserRoleUser}
	expired, err := SignToken(secret, user, now.Add(-time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	valid, err := SignToken(secret, user, now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(valid, ".")
	if parts[2][0] == 'A' {
		parts[2] = "B" + parts[2][1:]
	} else {
		parts[2] = "A" + parts[2][1:]
	}
	tampered := strings.Join(parts, ".")

	for _, test := range []struct {
		name  string
		token string
	}{
		{name: "expired", token: expired},
		{name: "tampered signature", token: tampered},
		{name: "malformed", token: "not-a-jwt"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := ParseToken(secret, test.token, now); err == nil {
				t.Fatal("ParseToken() succeeded, want an error")
			}
		})
	}
}

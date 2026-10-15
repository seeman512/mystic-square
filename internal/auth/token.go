package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"mystic-square/internal/model"
)

var errInvalidToken = errors.New("invalid token")

type jwtHeader struct {
	Algorithm string `json:"alg"`
	Type      string `json:"typ"`
}

// Claims contains the identity and authorization data carried by a JWT.
type Claims struct {
	UserID uint64         `json:"user_id"`
	Role   model.UserRole `json:"role"`
	Expiry int64          `json:"exp"`
}

// SignToken creates an HS256 JWT for user that expires at expiresAt.
func SignToken(secret string, user model.User, expiresAt time.Time) (string, error) {
	if err := validateSecret(secret); err != nil {
		return "", err
	}
	if user.ID == 0 || (user.Role != model.UserRoleAdmin && user.Role != model.UserRoleUser) {
		return "", errInvalidToken
	}

	header, err := json.Marshal(jwtHeader{Algorithm: "HS256", Type: "JWT"})
	if err != nil {
		return "", fmt.Errorf("encode JWT header: %w", err)
	}
	payload, err := json.Marshal(Claims{UserID: user.ID, Role: user.Role, Expiry: expiresAt.Unix()})
	if err != nil {
		return "", fmt.Errorf("encode JWT claims: %w", err)
	}
	unsigned := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
	signature, err := sign(secret, unsigned)
	if err != nil {
		return "", fmt.Errorf("sign JWT: %w", err)
	}
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

// ParseToken validates an HS256 JWT and returns its claims if it has not expired.
func ParseToken(secret, token string, now time.Time) (Claims, error) {
	if err := validateSecret(secret); err != nil {
		return Claims{}, err
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return Claims{}, errInvalidToken
	}

	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return Claims{}, errInvalidToken
	}
	var header jwtHeader
	if err := json.Unmarshal(headerBytes, &header); err != nil || header.Algorithm != "HS256" || header.Type != "JWT" {
		return Claims{}, errInvalidToken
	}

	providedSignature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return Claims{}, errInvalidToken
	}
	expectedSignature, err := sign(secret, parts[0]+"."+parts[1])
	if err != nil || !hmac.Equal(providedSignature, expectedSignature) {
		return Claims{}, errInvalidToken
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Claims{}, errInvalidToken
	}
	var claims Claims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil || claims.UserID == 0 ||
		(claims.Role != model.UserRoleAdmin && claims.Role != model.UserRoleUser) || claims.Expiry == 0 {
		return Claims{}, errInvalidToken
	}
	if !now.Before(time.Unix(claims.Expiry, 0)) {
		return Claims{}, errInvalidToken
	}
	return claims, nil
}

func sign(secret, message string) ([]byte, error) {
	mac := hmac.New(sha256.New, []byte(secret))
	if _, err := mac.Write([]byte(message)); err != nil {
		return nil, err
	}
	return mac.Sum(nil), nil
}

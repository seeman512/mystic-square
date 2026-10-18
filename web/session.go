package web

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"time"

	"mystic-square/internal/auth"
	"mystic-square/internal/model"
)

func (h *Handler) ensureCSRFToken(w http.ResponseWriter, r *http.Request) string {
	if token := csrfTokenFromRequest(r); token != "" {
		return token
	}
	bytesValue := make([]byte, 32)
	if _, err := rand.Read(bytesValue); err != nil {
		h.logger.Error("generate csrf token", "error", err)
		return ""
	}
	token := base64.RawURLEncoding.EncodeToString(bytesValue)
	http.SetCookie(w, &http.Cookie{
		Name:     CSRFTokenCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.secureCookie,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(auth.TokenLifetime / time.Second),
		Expires:  h.now().UTC().Add(auth.TokenLifetime),
	})
	return token
}

func csrfTokenFromRequest(r *http.Request) string {
	cookie, err := r.Cookie(CSRFTokenCookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}

func (h *Handler) validCSRF(r *http.Request) bool {
	cookieToken := csrfTokenFromRequest(r)
	if cookieToken == "" {
		return false
	}
	if r.PostFormValue("csrf_token") != "" {
		formToken := r.PostFormValue("csrf_token")
		return subtle.ConstantTimeCompare([]byte(cookieToken), []byte(formToken)) == 1
	}
	headerToken := r.Header.Get("X-CSRF-Token")
	return subtle.ConstantTimeCompare([]byte(cookieToken), []byte(headerToken)) == 1
}

func (h *Handler) currentUser(r *http.Request) (model.User, bool) {
	cookie, err := r.Cookie(SessionCookieName)
	if err != nil {
		return model.User{}, false
	}
	claims, err := auth.ParseToken(h.jwtSecret, cookie.Value, h.now())
	if err != nil {
		return model.User{}, false
	}
	user, err := h.users.Get(r.Context(), claims.UserID)
	if err != nil {
		return model.User{}, false
	}
	return user, true
}

func (h *Handler) setSession(w http.ResponseWriter, token string) {
	now := h.now().UTC()
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.secureCookie,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(auth.TokenLifetime / time.Second),
		Expires:  now.Add(auth.TokenLifetime),
	})
}

func (h *Handler) expireSession(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   h.secureCookie,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
		Expires:  time.Unix(1, 0).UTC(),
	})
}

func (h *Handler) expireSessionIfPresent(w http.ResponseWriter, r *http.Request) {
	if _, err := r.Cookie(SessionCookieName); err == nil {
		h.expireSession(w)
	}
}

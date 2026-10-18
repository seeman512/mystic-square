package web

import (
	"errors"
	"net/http"
	"strings"

	"mystic-square/internal/auth"
	"mystic-square/internal/repository"
)

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if _, ok := h.currentUser(r); ok {
			h.redirect(w, r, "/game")
			return
		}
		h.renderPage(w, r, "login", h.pageData("Login", r, PageData{IsLogin: true}), http.StatusOK)
	case http.MethodPost:
		h.handleLogin(w, r)
	default:
		h.methodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if _, ok := h.currentUser(r); ok {
			h.redirect(w, r, "/game")
			return
		}
		h.renderPage(w, r, "register", h.pageData("Register", r, PageData{IsRegister: true}), http.StatusOK)
	case http.MethodPost:
		h.handleRegister(w, r)
	default:
		h.methodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

func (h *Handler) handleLogin(w http.ResponseWriter, r *http.Request) {
	email, password, err := loginPayload(w, r)
	if err != nil {
		h.renderFormError(w, r, "login", PageData{IsLogin: true}, http.StatusUnprocessableEntity,
			"Please enter a valid email and password.")
		return
	}

	email = strings.TrimSpace(email)
	_, token, err := h.service.Login(r.Context(), email, password)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrInvalidLogin), errors.Is(err, auth.ErrInvalidCredentials):
			h.renderFormError(w, r, "login", PageData{
				IsLogin: true,
				Email:   email,
			}, http.StatusUnprocessableEntity, "Invalid email or password")
		case errors.Is(err, auth.ErrInvalidSecret):
			h.renderFormError(w, r, "login", PageData{IsLogin: true, Email: email}, http.StatusInternalServerError,
				"Authentication is not configured. Please try again later.")
		default:
			h.logger.Error("web login failed", "error", err)
			h.renderFormError(w, r, "login", PageData{IsLogin: true, Email: email}, http.StatusInternalServerError,
				"We could not sign you in. Please try again later.")
		}
		return
	}

	h.setSession(w, token)
	h.redirect(w, r, "/game")
}

func (h *Handler) handleRegister(w http.ResponseWriter, r *http.Request) {
	email, fullName, password, err := registerPayload(w, r)
	if err != nil {
		h.renderFormError(w, r, "register", PageData{IsRegister: true}, http.StatusUnprocessableEntity,
			"Please provide a valid name, email, and password.")
		return
	}

	email = strings.TrimSpace(email)
	fullName = strings.TrimSpace(fullName)
	_, token, err := h.service.Register(r.Context(), email, fullName, password)
	if err != nil {
		data := PageData{
			IsRegister:    true,
			Email:         email,
			FullNameInput: fullName,
		}
		switch {
		case errors.Is(err, auth.ErrInvalidRegistration):
			h.renderFormError(w, r, "register", data, http.StatusUnprocessableEntity,
				"Please provide a valid name, email, and password (passwords must be 8-72 characters).")
		case errors.Is(err, repository.ErrConflict):
			h.renderFormError(w, r, "register", data, http.StatusUnprocessableEntity,
				"An account with that email already exists.")
		case errors.Is(err, auth.ErrInvalidSecret):
			h.renderFormError(w, r, "register", data, http.StatusInternalServerError,
				"Authentication is not configured. Please try again later.")
		default:
			h.logger.Error("web registration failed", "error", err)
			h.renderFormError(w, r, "register", data, http.StatusInternalServerError,
				"We could not create your account. Please try again later.")
		}
		return
	}

	h.setSession(w, token)
	h.redirect(w, r, "/game")
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type registerRequest struct {
	Email    string `json:"email"`
	FullName string `json:"full_name"`
	Password string `json:"password"`
}

func loginPayload(w http.ResponseWriter, r *http.Request) (string, string, error) {
	if isJSONRequest(r) {
		var input loginRequest
		if err := decodeJSONBody(w, r, &input); err != nil {
			return "", "", err
		}
		return input.Email, input.Password, nil
	}

	values, err := parseForm(w, r)
	if err != nil {
		return "", "", err
	}
	return values.Get("email"), values.Get("password"), nil
}

func registerPayload(w http.ResponseWriter, r *http.Request) (string, string, string, error) {
	if isJSONRequest(r) {
		var input registerRequest
		if err := decodeJSONBody(w, r, &input); err != nil {
			return "", "", "", err
		}
		return input.Email, input.FullName, input.Password, nil
	}

	values, err := parseForm(w, r)
	if err != nil {
		return "", "", "", err
	}
	return values.Get("email"), values.Get("full_name"), values.Get("password"), nil
}

package web

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strings"
	"time"

	"mystic-square/internal/auth"
	"mystic-square/internal/model"
	"mystic-square/internal/repository"
)

const (
	// SessionCookieName is the name of the browser-only authentication cookie.
	SessionCookieName = "mystic_square_session"
	maxAuthBodySize   = 1 << 20
	dateFormat        = "2006-01-02"
)

// Config controls the web authentication and rendering behavior.
type Config struct {
	// JWTSecret must be the same secret used by the JSON API.
	JWTSecret string
	// SecureCookie should be true when the application is served over HTTPS.
	SecureCookie bool
	// Logger receives template and response-write errors. A nil logger uses
	// slog.Default.
	Logger *slog.Logger
	// Now is used for cookie expiry and the server-rendered footer date. A nil
	// value uses time.Now.
	Now func() time.Time
}

// PageData is the deliberately small view model exposed to the templates.
// It does not contain tokens, password hashes, or the complete User value.
type PageData struct {
	Title         string
	Date          string
	Authenticated bool
	FullName      string
	Email         string
	FullNameInput string
	Errors        []string
	Error         string
	IsLogin       bool
	IsRegister    bool
	IsWelcome     bool
}

// Handler serves the HTML pages and embedded static assets for the web UI.
type Handler struct {
	service       *auth.Service
	users         repository.UserRepository
	jwtSecret     string
	secureCookie  bool
	logger        *slog.Logger
	now           func() time.Time
	mux           *http.ServeMux
	baseTemplates *template.Template
	pageTemplates map[string]*template.Template
}

// NewHandler creates the embedded-template web handler.
func NewHandler(service *auth.Service, users repository.UserRepository, config Config) (*Handler, error) {
	if service == nil {
		return nil, errors.New("web: authentication service is nil")
	}
	if users == nil {
		return nil, errors.New("web: user repository is nil")
	}
	logger := config.Logger
	if logger == nil {
		logger = slog.Default()
	}
	now := config.Now
	if now == nil {
		now = time.Now
	}

	base, err := template.ParseFS(TemplatesFS, "layout.html", "partials/header.html", "partials/footer.html")
	if err != nil {
		return nil, fmt.Errorf("parse shared web templates: %w", err)
	}
	pages := make(map[string]*template.Template, 4)
	for _, page := range []string{"login", "register", "welcome", "logout"} {
		pageTemplates, err := base.Clone()
		if err != nil {
			return nil, fmt.Errorf("clone templates for %s: %w", page, err)
		}
		pageTemplates, err = pageTemplates.ParseFS(TemplatesFS, page+".html")
		if err != nil {
			return nil, fmt.Errorf("parse %s template: %w", page, err)
		}
		pages[page] = pageTemplates
	}

	h := &Handler{
		service:       service,
		users:         users,
		jwtSecret:     config.JWTSecret,
		secureCookie:  config.SecureCookie,
		logger:        logger,
		now:           now,
		baseTemplates: base,
		pageTemplates: pages,
	}
	h.mux = http.NewServeMux()
	h.mux.HandleFunc("/login", h.login)
	h.mux.HandleFunc("/register", h.register)
	h.mux.HandleFunc("/welcome", h.welcome)
	h.mux.HandleFunc("/logout", h.logout)
	h.mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(StaticFS))))
	return h, nil
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if _, ok := h.currentUser(r); ok {
			h.redirect(w, r, "/welcome")
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
			h.redirect(w, r, "/welcome")
			return
		}
		h.renderPage(w, r, "register", h.pageData("Register", r, PageData{IsRegister: true}), http.StatusOK)
	case http.MethodPost:
		h.handleRegister(w, r)
	default:
		h.methodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

func (h *Handler) welcome(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.methodNotAllowed(w, http.MethodGet)
		return
	}
	user, ok := h.currentUser(r)
	if !ok {
		h.expireSessionIfPresent(w, r)
		h.redirect(w, r, "/login")
		return
	}
	data := h.pageData("Welcome", r, PageData{
		Authenticated: true,
		FullName:      user.FullName,
		IsWelcome:     true,
	})
	h.renderPage(w, r, "welcome", data, http.StatusOK)
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// GET is intentionally idempotent and does not clear state. The POST
		// action has already expired the cookie before redirecting here.
		h.renderPage(w, r, "logout", h.pageData("Goodbye", r, PageData{}), http.StatusOK)
	case http.MethodPost:
		h.expireSession(w)
		h.redirect(w, r, "/logout")
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
	h.redirect(w, r, "/welcome")
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
	h.redirect(w, r, "/welcome")
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

func isJSONRequest(r *http.Request) bool {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	return err == nil && mediaType == "application/json"
}

func decodeJSONBody(w http.ResponseWriter, r *http.Request, value any) error {
	if r.Body == nil {
		return io.EOF
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxAuthBodySize))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}

	var extra json.RawMessage
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}

func parseForm(w http.ResponseWriter, r *http.Request) (mapValues, error) {
	if r.Body != nil {
		r.Body = http.MaxBytesReader(w, r.Body, maxAuthBodySize)
	}
	if err := r.ParseForm(); err != nil {
		return nil, err
	}
	return mapValues(r.Form), nil
}

// mapValues keeps the handler independent of the mutable Request.Form map and
// exposes only the read operation needed by the authentication forms.
type mapValues map[string][]string

func (values mapValues) Get(key string) string {
	items := values[key]
	if len(items) == 0 {
		return ""
	}
	return items[0]
}

func (h *Handler) pageData(title string, r *http.Request, data PageData) PageData {
	data.Title = title
	data.Date = h.now().UTC().Format(dateFormat)
	if !data.Authenticated {
		// Login, register, and logout are deliberately guest pages. The only
		// pages that opt into the authenticated header do so explicitly above.
		data.FullName = ""
	}
	return data
}

func (h *Handler) renderFormError(w http.ResponseWriter, r *http.Request, page string, data PageData, status int, message string) {
	data.Errors = []string{message}
	h.renderPage(w, r, page, h.pageData(pageTitle(page), r, data), status)
}

func pageTitle(page string) string {
	if page == "register" {
		return "Register"
	}
	return "Login"
}

func (h *Handler) renderPage(w http.ResponseWriter, r *http.Request, page string, data PageData, status int) {
	// Both full-page and form-fragment responses can be returned by these
	// routes, so caches must distinguish them by HX-Request.
	w.Header().Add("Vary", "HX-Request")

	tmpl := h.pageTemplates[page]
	name := "layout"
	if isHTMXRequest(r) && (page == "login" || page == "register") && status >= http.StatusUnprocessableEntity {
		name = page + "-form"
	}
	if tmpl == nil {
		h.logger.Error("web template is missing", "page", page)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	var body bytes.Buffer
	if err := tmpl.ExecuteTemplate(&body, name, data); err != nil {
		h.logger.Error("execute web template", "page", page, "template", name, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if _, err := io.Copy(w, &body); err != nil {
		h.logger.Error("write web response", "page", page, "error", err)
	}
}

func (h *Handler) renderError(w http.ResponseWriter, r *http.Request, title, message string, status int) {
	w.Header().Add("Vary", "HX-Request")
	data := h.pageData(title, r, PageData{Error: message})
	var body bytes.Buffer
	if err := h.baseTemplates.ExecuteTemplate(&body, "layout", data); err != nil {
		h.logger.Error("execute web error template", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if _, err := io.Copy(w, &body); err != nil {
		h.logger.Error("write web error response", "error", err)
	}
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

func (h *Handler) redirect(w http.ResponseWriter, r *http.Request, location string) {
	w.Header().Add("Vary", "HX-Request")
	if isHTMXRequest(r) {
		w.Header().Set("HX-Redirect", location)
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, location, http.StatusSeeOther)
}

func isHTMXRequest(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("HX-Request"), "true")
}

func (h *Handler) methodNotAllowed(w http.ResponseWriter, methods ...string) {
	w.Header().Set("Allow", strings.Join(methods, ", "))
	h.renderError(w, &http.Request{}, "Method Not Allowed", "method not allowed", http.StatusMethodNotAllowed)
}

var _ http.Handler = (*Handler)(nil)

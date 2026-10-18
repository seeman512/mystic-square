package web

import (
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"time"

	"mystic-square/internal/auth"
	"mystic-square/internal/repository"
)

const (
	// SessionCookieName is the name of the browser-only authentication cookie.
	SessionCookieName   = "mystic_square_session"
	CSRFTokenCookieName = "mystic_square_csrf"
	maxAuthBodySize     = 1 << 20
	dateFormat          = "2006-01-02"
)

// Config controls the web authentication and rendering behavior.
type Config struct {
	// JWTSecret must be the same secret used by the JSON API.
	JWTSecret string
	// Levels and Games provide the services used by the authenticated game page.
	Levels LevelLister
	Games  GamePageService
	// SecureCookie should be true when the application is served over HTTPS.
	SecureCookie bool
	// Logger receives template and response-write errors. A nil logger uses
	// slog.Default.
	Logger *slog.Logger
	// Now is used for cookie expiry and the server-rendered footer date. A nil
	// value uses time.Now.
	Now func() time.Time
}

// Handler serves the HTML pages and embedded static assets for the web UI.
type Handler struct {
	service       *auth.Service
	users         repository.UserRepository
	levels        LevelLister
	games         GamePageService
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
	for _, page := range []string{"login", "register", "game", "logout"} {
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
		levels:        config.Levels,
		games:         config.Games,
		jwtSecret:     config.JWTSecret,
		secureCookie:  config.SecureCookie,
		logger:        logger,
		now:           now,
		baseTemplates: base,
		pageTemplates: pages,
	}
	h.mux = http.NewServeMux()
	h.registerRoutes()
	return h, nil
}

func (h *Handler) registerRoutes() {
	h.mux.HandleFunc("/login", h.login)
	h.mux.HandleFunc("/register", h.register)
	h.mux.HandleFunc("/game", h.game)
	h.mux.HandleFunc("/game/start", h.gameStart)
	h.mux.HandleFunc("/game/", h.gameMove)
	h.mux.HandleFunc("/logout", h.logout)
	h.mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(StaticFS))))
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

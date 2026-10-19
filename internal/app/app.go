// Package app wires the HTTP API together.
package app

import (
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "mystic-square/docs" // registers the generated OpenAPI spec
	"mystic-square/internal/auth"
	"mystic-square/internal/handler"
	"mystic-square/internal/middleware"
	"mystic-square/internal/model"
	"mystic-square/internal/repository"
	"mystic-square/internal/service"
	"mystic-square/web"
)

// NewRouter returns a fully configured HTTP handler with fresh in-memory storage.
// It is kept as a convenient isolated router for unit tests and local demos.
func NewRouter() http.Handler {
	return NewRouterWithRepositories(
		repository.NewInMemoryLevelRepository(),
		repository.NewInMemoryUserRepository(),
		repository.NewInMemoryGameRepository(),
	)
}

// NewRouterWithLevelRepository wires the HTTP API to the supplied level store.
// User and game operations use isolated in-memory stores for compatibility with
// the level-only constructor.
func NewRouterWithLevelRepository(levelRepository repository.LevelRepository) http.Handler {
	return NewRouterWithRepositories(
		levelRepository,
		repository.NewInMemoryUserRepository(),
		repository.NewInMemoryGameRepository(),
	)
}

// NewRouterWithRepositories wires the complete API to its storage backends.
func NewRouterWithRepositories(
	levelRepository repository.LevelRepository,
	userRepository repository.UserRepository,
	gameRepository repository.GameRepository,
) http.Handler {
	if levelRepository == nil {
		levelRepository = repository.NewInMemoryLevelRepository()
	}
	if userRepository == nil {
		userRepository = repository.NewInMemoryUserRepository()
	}
	if gameRepository == nil {
		gameRepository = repository.NewInMemoryGameRepository()
	}
	levelService := service.NewLevelService(levelRepository)
	userService := service.NewUserService(userRepository)
	gameService := service.NewGameService(gameRepository, levelRepository, userRepository)
	levelHandler := handler.NewLevelHandler(levelService)
	userHandler := handler.NewUserHandler(userService)
	gameHandler := handler.NewGameHandler(gameService)
	jwtSecret := os.Getenv("JWT_SECRET")
	authService := auth.NewService(userRepository, jwtSecret)
	authHandler := handler.NewAuthHandler(authService)
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	webHandler, err := web.NewHandler(authService, userRepository, web.Config{
		JWTSecret:    jwtSecret,
		Levels:       levelService,
		Games:        gameService,
		AdminLevels:  levelService,
		AdminUsers:   userService,
		SecureCookie: webCookieSecure(),
		Logger:       logger,
	})
	if err != nil {
		panic(err)
	}

	router := gin.New()
	router.Use(
		middleware.Recovery(logger),
		middleware.Logging(logger),
		middleware.CORS(),
	)

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	api := router.Group("/api/v1")
	public := api.Group("")
	authHandler.RegisterRoutes(public)
	levelHandler.RegisterPublicRoutes(public)

	protected := api.Group("")
	protected.Use(middleware.AuthMiddleware(jwtSecret))
	gameHandler.RegisterRoutes(protected)

	admin := protected.Group("")
	admin.Use(middleware.RequireRole(string(model.UserRoleAdmin)))
	levelHandler.RegisterAdminRoutes(admin)
	userHandler.RegisterRoutes(admin)

	// The web UI has its own cookie-based authentication layer. It is mounted
	// separately from the API so Bearer-token middleware and JSON contracts are
	// unchanged.
	router.GET("/", gin.WrapH(webHandler))
	router.GET("/login", gin.WrapH(webHandler))
	router.POST("/login", gin.WrapH(webHandler))
	router.GET("/register", gin.WrapH(webHandler))
	router.POST("/register", gin.WrapH(webHandler))
	router.GET("/game", gin.WrapH(webHandler))
	router.POST("/game/start", gin.WrapH(webHandler))
	router.GET("/levels", gin.WrapH(webHandler))
	router.POST("/levels", gin.WrapH(webHandler))
	router.POST("/levels/:id", gin.WrapH(webHandler))
	router.GET("/users", gin.WrapH(webHandler))
	router.POST("/users/:id/role", gin.WrapH(webHandler))
	router.GET("/game/:id/move", gin.WrapH(webHandler))
	router.POST("/game/:id/move", gin.WrapH(webHandler))
	router.GET("/logout", gin.WrapH(webHandler))
	router.POST("/logout", gin.WrapH(webHandler))
	router.GET("/static/*path", gin.WrapH(webHandler))

	router.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, model.NewErrorResponse("not_found", "route not found"))
	})
	router.NoMethod(func(c *gin.Context) {
		c.JSON(http.StatusMethodNotAllowed, model.NewErrorResponse("method_not_allowed", "method not allowed"))
	})
	return router
}

func webCookieSecure() bool {
	if value := os.Getenv("WEB_COOKIE_SECURE"); value != "" {
		secure, err := strconv.ParseBool(value)
		if err == nil {
			return secure
		}
	}
	appEnvironment := strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV")))
	return appEnvironment == "production" || appEnvironment == "prod"
}

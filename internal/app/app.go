// Package app wires the HTTP API together.
package app

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "mystic-square/docs" // registers the generated OpenAPI spec
	"mystic-square/internal/handler"
	"mystic-square/internal/middleware"
	"mystic-square/internal/model"
	"mystic-square/internal/repository"
	"mystic-square/internal/service"
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
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))

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
	levelHandler.RegisterRoutes(api)
	userHandler.RegisterRoutes(api)
	gameHandler.RegisterRoutes(api)

	router.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, model.NewErrorResponse("not_found", "route not found"))
	})
	router.NoMethod(func(c *gin.Context) {
		c.JSON(http.StatusMethodNotAllowed, model.NewErrorResponse("method_not_allowed", "method not allowed"))
	})
	return router
}

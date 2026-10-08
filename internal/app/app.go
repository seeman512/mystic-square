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
func NewRouter() http.Handler {
	levelRepository := repository.NewInMemoryLevelRepository()
	levelService := service.NewLevelService(levelRepository)
	levelHandler := handler.NewLevelHandler(levelService)
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

	router.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, model.NewErrorResponse("not_found", "route not found"))
	})
	router.NoMethod(func(c *gin.Context) {
		c.JSON(http.StatusMethodNotAllowed, model.NewErrorResponse("method_not_allowed", "method not allowed"))
	})
	return router
}

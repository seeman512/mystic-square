package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"

	"mystic-square/internal/app"
	"mystic-square/internal/repository"
)

// @title			Mystic Square API
// @version		1.0
// @description	HTTP API for Mystic Square levels, users, and sliding-puzzle games.
// @schemes		http
// @host			localhost:8080
// @BasePath		/api/v1
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer " followed by a JWT token.
func main() {
	if len([]byte(os.Getenv("JWT_SECRET"))) < 32 {
		log.Fatal("JWT_SECRET must be set to at least 32 bytes")
	}

	addr := ":8080"
	if port := os.Getenv("PORT"); port != "" {
		addr = ":" + port
	}

	var router http.Handler
	if databaseURL := configuredDatabaseURL(); databaseURL != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		db, err := repository.OpenDatabase(ctx, databaseURL)
		cancel()
		if err != nil {
			log.Fatal(err)
		}
		sqlDB, err := db.DB()
		if err != nil {
			log.Fatal(err)
		}
		defer func() {
			if err := sqlDB.Close(); err != nil {
				log.Printf("close database: %v", err)
			}
		}()
		router = app.NewRouterWithRepositories(
			repository.NewGORMLevelRepository(db),
			repository.NewGORMUserRepository(db),
			repository.NewGORMGameRepository(db),
		)
		log.Printf("using PostgreSQL database; apply migrations before starting the API")
	} else {
		router = app.NewRouter()
		log.Printf("DATABASE_URL is not set; using in-memory storage")
	}

	log.Printf("listening on %s", addr)
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatal(err)
	}
}

func configuredDatabaseURL() string {
	if databaseURL := os.Getenv("DATABASE_URL"); databaseURL != "" {
		return databaseURL
	}

	user := os.Getenv("PG_USER")
	if user == "" {
		return ""
	}
	database := os.Getenv("PG_DB")
	if database == "" {
		database = user
	}
	host := environmentOr("PG_HOST", "localhost")
	port := environmentOr("PG_PORT", "5432")
	sslmode := environmentOr("PG_SSLMODE", "disable")

	dsn := url.URL{
		Scheme:   "postgres",
		Host:     net.JoinHostPort(host, port),
		Path:     "/" + database,
		RawQuery: url.Values{"sslmode": {sslmode}}.Encode(),
	}
	if password := os.Getenv("PG_PASSWORD"); password != "" {
		dsn.User = url.UserPassword(user, password)
	} else {
		dsn.User = url.User(user)
	}
	return dsn.String()
}

func environmentOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

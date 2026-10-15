package repository

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormLogger "gorm.io/gorm/logger"
)

const (
	defaultMaxOpenConns = 10
	defaultMaxIdleConns = 5
)

// OpenDatabase opens a PostgreSQL database and verifies the connection.
// Schema changes are intentionally not performed here; apply migrations with
// golang-migrate before starting the API.
func OpenDatabase(ctx context.Context, url string) (*gorm.DB, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if url == "" {
		return nil, fmt.Errorf("database URL is required")
	}

	db, err := gorm.Open(postgres.Open(url), &gorm.Config{
		Logger: gormLogger.New(log.New(os.Stderr, "", log.LstdFlags), gormLogger.Config{
			SlowThreshold:        time.Second,
			LogLevel:             gormLogger.Warn,
			ParameterizedQueries: true,
			Colorful:             false,
		}),
		TranslateError: true,
	})
	if err != nil {
		return nil, fmt.Errorf("open PostgreSQL database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get PostgreSQL connection pool: %w", err)
	}
	configurePool(sqlDB)
	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("ping PostgreSQL database: %w", err)
	}
	return db, nil
}

func configurePool(db *sql.DB) {
	db.SetMaxOpenConns(defaultMaxOpenConns)
	db.SetMaxIdleConns(defaultMaxIdleConns)
	db.SetConnMaxLifetime(30 * time.Minute)
	db.SetConnMaxIdleTime(5 * time.Minute)
}

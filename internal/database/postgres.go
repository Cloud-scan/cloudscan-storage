package database

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
	log "github.com/sirupsen/logrus"
)

// DB wraps sql.DB with additional methods
type DB struct {
	*sql.DB
}

// NewPostgresDB creates a new PostgreSQL database connection
func NewPostgresDB(dsn string, maxConns, minConns int) (*DB, error) {
	log.WithField("dsn", maskPassword(dsn)).Info("Connecting to PostgreSQL")

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(maxConns)
	db.SetMaxIdleConns(minConns)
	db.SetConnMaxLifetime(time.Hour)
	db.SetConnMaxIdleTime(30 * time.Minute)

	// Ping database to verify connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	log.Info("PostgreSQL connection established")
	return &DB{db}, nil
}

// RunMigrations runs database migrations
func RunMigrations(db *sql.DB, migrationsPath string) error {
	log.WithField("path", migrationsPath).Info("Running database migrations")

	// Database migrations are handled externally via Kubernetes migration job
	// See migrations/001_initial_schema.up.sql
	// In production, use a migration tool like golang-migrate or goose

	log.Info("Database migrations completed (migrations handled externally)")
	return nil
}

// maskPassword masks the password in the DSN for logging
func maskPassword(dsn string) string {
	// Simple password masking - in production use a proper URL parser
	return "postgresql://***:***@***"
}
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

	// For now, we'll execute migrations inline
	// In production, use a migration tool like golang-migrate

	migrations := []string{
		`
		CREATE TABLE IF NOT EXISTS artifacts (
			id UUID PRIMARY KEY,
			scan_id UUID NOT NULL,
			organization_id UUID NOT NULL,
			type VARCHAR(50) NOT NULL,
			filename VARCHAR(500) NOT NULL,
			size_bytes BIGINT NOT NULL DEFAULT 0,
			content_type VARCHAR(100),
			storage_path VARCHAR(1000) NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
			expires_at TIMESTAMP WITH TIME ZONE,
			deleted_at TIMESTAMP WITH TIME ZONE,
			INDEX idx_artifacts_scan_id (scan_id),
			INDEX idx_artifacts_org_id (organization_id),
			INDEX idx_artifacts_type (type),
			INDEX idx_artifacts_expires_at (expires_at) WHERE deleted_at IS NULL,
			INDEX idx_artifacts_deleted_at (deleted_at)
		);
		`,
	}

	for i, migration := range migrations {
		log.WithField("migration", i+1).Debug("Executing migration")
		if _, err := db.Exec(migration); err != nil {
			return fmt.Errorf("migration %d failed: %w", i+1, err)
		}
	}

	log.Info("Database migrations completed successfully")
	return nil
}

// maskPassword masks the password in the DSN for logging
func maskPassword(dsn string) string {
	// Simple password masking - in production use a proper URL parser
	return "postgresql://***:***@***"
}
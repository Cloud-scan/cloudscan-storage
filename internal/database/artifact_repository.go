package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/cloud-scan/cloudscan-storage/internal/domain"
	"github.com/cloud-scan/cloudscan-storage/internal/interfaces"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

// ArtifactRepository implements interfaces.ArtifactRepository using PostgreSQL
type ArtifactRepository struct {
	db     *DB
	logger *log.Entry
}

// NewArtifactRepository creates a new artifact repository
func NewArtifactRepository(db *DB) interfaces.ArtifactRepository {
	return &ArtifactRepository{
		db:     db,
		logger: log.WithField("component", "artifact-repository"),
	}
}

// Create creates a new artifact record
func (r *ArtifactRepository) Create(ctx context.Context, artifact *domain.Artifact) error {
	logger := r.logger.WithField("artifact_id", artifact.ID.String())
	logger.Debug("Creating artifact record")

	query := `
		INSERT INTO artifacts (
			id, scan_id, organization_id, type, filename,
			size_bytes, content_type, storage_path, created_at, expires_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	_, err := r.db.ExecContext(ctx, query,
		artifact.ID,
		artifact.ScanID,
		artifact.OrganizationID,
		artifact.Type,
		artifact.Filename,
		artifact.SizeBytes,
		artifact.ContentType,
		artifact.StoragePath,
		artifact.CreatedAt,
		artifact.ExpiresAt,
	)

	if err != nil {
		logger.WithError(err).Error("Failed to create artifact record")
		return fmt.Errorf("failed to create artifact: %w", err)
	}

	logger.Info("Artifact record created successfully")
	return nil
}

// Get retrieves an artifact by ID
func (r *ArtifactRepository) Get(ctx context.Context, id string) (*domain.Artifact, error) {
	logger := r.logger.WithField("artifact_id", id)
	logger.Debug("Getting artifact record")

	artifactID, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("invalid artifact ID: %w", err)
	}

	query := `
		SELECT id, scan_id, organization_id, type, filename,
		       size_bytes, content_type, storage_path, created_at, expires_at
		FROM artifacts
		WHERE id = $1 AND deleted_at IS NULL
	`

	artifact := &domain.Artifact{}
	err = r.db.QueryRowContext(ctx, query, artifactID).Scan(
		&artifact.ID,
		&artifact.ScanID,
		&artifact.OrganizationID,
		&artifact.Type,
		&artifact.Filename,
		&artifact.SizeBytes,
		&artifact.ContentType,
		&artifact.StoragePath,
		&artifact.CreatedAt,
		&artifact.ExpiresAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("artifact not found: %s", id)
		}
		logger.WithError(err).Error("Failed to get artifact record")
		return nil, fmt.Errorf("failed to get artifact: %w", err)
	}

	return artifact, nil
}

// List retrieves artifacts with filters
func (r *ArtifactRepository) List(ctx context.Context, filter interfaces.ArtifactFilter) ([]*domain.Artifact, error) {
	r.logger.Debug("Listing artifacts with filters")

	// Build query with filters
	query := `
		SELECT id, scan_id, organization_id, type, filename,
		       size_bytes, content_type, storage_path, created_at, expires_at
		FROM artifacts
		WHERE deleted_at IS NULL
	`

	var args []interface{}
	argIndex := 1

	if filter.ScanID != nil {
		scanID, err := uuid.Parse(*filter.ScanID)
		if err != nil {
			return nil, fmt.Errorf("invalid scan_id: %w", err)
		}
		query += fmt.Sprintf(" AND scan_id = $%d", argIndex)
		args = append(args, scanID)
		argIndex++
	}

	if filter.OrganizationID != nil {
		orgID, err := uuid.Parse(*filter.OrganizationID)
		if err != nil {
			return nil, fmt.Errorf("invalid organization_id: %w", err)
		}
		query += fmt.Sprintf(" AND organization_id = $%d", argIndex)
		args = append(args, orgID)
		argIndex++
	}

	if filter.Type != nil {
		query += fmt.Sprintf(" AND type = $%d", argIndex)
		args = append(args, string(*filter.Type))
		argIndex++
	}

	// Add ordering
	query += " ORDER BY created_at DESC"

	// Add pagination
	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argIndex)
		args = append(args, filter.Limit)
		argIndex++
	}

	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argIndex)
		args = append(args, filter.Offset)
		argIndex++
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		r.logger.WithError(err).Error("Failed to list artifacts")
		return nil, fmt.Errorf("failed to list artifacts: %w", err)
	}
	defer rows.Close()

	var artifacts []*domain.Artifact
	for rows.Next() {
		artifact := &domain.Artifact{}
		err := rows.Scan(
			&artifact.ID,
			&artifact.ScanID,
			&artifact.OrganizationID,
			&artifact.Type,
			&artifact.Filename,
			&artifact.SizeBytes,
			&artifact.ContentType,
			&artifact.StoragePath,
			&artifact.CreatedAt,
			&artifact.ExpiresAt,
		)
		if err != nil {
			r.logger.WithError(err).Error("Failed to scan artifact row")
			return nil, fmt.Errorf("failed to scan artifact: %w", err)
		}
		artifacts = append(artifacts, artifact)
	}

	if err = rows.Err(); err != nil {
		r.logger.WithError(err).Error("Error iterating artifact rows")
		return nil, fmt.Errorf("error iterating artifacts: %w", err)
	}

	r.logger.WithField("count", len(artifacts)).Debug("Retrieved artifacts")
	return artifacts, nil
}

// Delete deletes an artifact record (soft delete)
func (r *ArtifactRepository) Delete(ctx context.Context, id string) error {
	logger := r.logger.WithField("artifact_id", id)
	logger.Debug("Deleting artifact record (soft delete)")

	artifactID, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("invalid artifact ID: %w", err)
	}

	query := `
		UPDATE artifacts
		SET deleted_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`

	result, err := r.db.ExecContext(ctx, query, artifactID)
	if err != nil {
		logger.WithError(err).Error("Failed to delete artifact record")
		return fmt.Errorf("failed to delete artifact: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("artifact not found or already deleted: %s", id)
	}

	logger.Info("Artifact record deleted successfully")
	return nil
}

// DeleteExpired deletes expired artifacts
func (r *ArtifactRepository) DeleteExpired(ctx context.Context) (int64, error) {
	r.logger.Debug("Deleting expired artifacts")

	query := `
		UPDATE artifacts
		SET deleted_at = NOW()
		WHERE expires_at IS NOT NULL
		  AND expires_at < NOW()
		  AND deleted_at IS NULL
	`

	result, err := r.db.ExecContext(ctx, query)
	if err != nil {
		r.logger.WithError(err).Error("Failed to delete expired artifacts")
		return 0, fmt.Errorf("failed to delete expired artifacts: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	r.logger.WithField("count", rowsAffected).Info("Deleted expired artifacts")
	return rowsAffected, nil
}
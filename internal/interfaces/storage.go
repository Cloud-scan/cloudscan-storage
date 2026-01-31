package interfaces

import (
	"context"
	"time"

	"github.com/cloud-scan/cloudscan-storage/internal/domain"
)

// StorageBackend defines the interface for object storage operations
type StorageBackend interface {
	// GeneratePresignedUploadURL creates a presigned URL for uploading an object
	GeneratePresignedUploadURL(ctx context.Context, path string, contentType string, expiresIn time.Duration) (*domain.PresignedURL, error)

	// GeneratePresignedDownloadURL creates a presigned URL for downloading an object
	GeneratePresignedDownloadURL(ctx context.Context, path string, expiresIn time.Duration) (*domain.PresignedURL, error)

	// DeleteObject deletes an object from storage
	DeleteObject(ctx context.Context, path string) error

	// ObjectExists checks if an object exists
	ObjectExists(ctx context.Context, path string) (bool, error)

	// GetObjectSize returns the size of an object in bytes
	GetObjectSize(ctx context.Context, path string) (int64, error)

	// Multipart upload operations (for large files)

	// InitiateMultipartUpload starts a multipart upload session
	InitiateMultipartUpload(ctx context.Context, path string) (string, error)

	// GetMultipartUploadPartURLs generates presigned URLs for multipart upload parts
	GetMultipartUploadPartURLs(ctx context.Context, path string, uploadID string, fromPart, numParts int, expiresIn time.Duration) ([]*domain.PresignedURL, error)

	// CompleteMultipartUpload finalizes a multipart upload
	CompleteMultipartUpload(ctx context.Context, path string, uploadID string, parts []CompletedPart) error

	// AbortMultipartUpload cancels a multipart upload
	AbortMultipartUpload(ctx context.Context, path string, uploadID string) error
}

// CompletedPart represents a completed multipart upload part
type CompletedPart struct {
	PartNumber int
	ETag       string
}

// ArtifactRepository defines the interface for artifact metadata persistence
type ArtifactRepository interface {
	// Create creates a new artifact record
	Create(ctx context.Context, artifact *domain.Artifact) error

	// Get retrieves an artifact by ID
	Get(ctx context.Context, id string) (*domain.Artifact, error)

	// List retrieves artifacts with filters
	List(ctx context.Context, filter ArtifactFilter) ([]*domain.Artifact, error)

	// Delete deletes an artifact record (soft delete)
	Delete(ctx context.Context, id string) error

	// DeleteExpired deletes expired artifacts
	DeleteExpired(ctx context.Context) (int64, error)
}

// ArtifactFilter represents filter criteria for listing artifacts
type ArtifactFilter struct {
	ScanID         *string
	OrganizationID *string
	Type           *domain.ArtifactType
	Limit          int
	Offset         int
}
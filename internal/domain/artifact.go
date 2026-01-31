package domain

import (
	"time"

	"github.com/google/uuid"
)

// Artifact represents a stored file/object
type Artifact struct {
	ID             uuid.UUID
	ScanID         uuid.UUID
	OrganizationID uuid.UUID
	Type           ArtifactType
	Filename       string
	SizeBytes      int64
	ContentType    string
	StoragePath    string
	CreatedAt      time.Time
	ExpiresAt      *time.Time
}

// ArtifactType represents the type of artifact
type ArtifactType string

const (
	ArtifactTypeSourceCode   ArtifactType = "source_code"
	ArtifactTypeScanResults  ArtifactType = "scan_results"
	ArtifactTypeReport       ArtifactType = "report"
	ArtifactTypeLog          ArtifactType = "log"
	ArtifactTypeTool         ArtifactType = "tool" // Scanner tool binaries
)

// PresignedURL represents a presigned URL with expiration
type PresignedURL struct {
	URL        string
	Headers    map[string]string
	ExpiresAt  time.Time
}
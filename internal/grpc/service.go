package grpc

import (
	"context"
	"time"

	"github.com/cloud-scan/cloudscan-storage/internal/domain"
	"github.com/cloud-scan/cloudscan-storage/internal/interfaces"
	pb "github.com/cloud-scan/cloudscan-storage/generated/proto"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// StorageServiceServer implements the gRPC StorageService interface
type StorageServiceServer struct {
	pb.UnimplementedStorageServiceServer
	storage          interfaces.StorageBackend
	artifactRepo     interfaces.ArtifactRepository
	defaultExpiration int // hours
	logger           *log.Entry
}

// NewStorageServiceServer creates a new gRPC storage service server
func NewStorageServiceServer(
	storage interfaces.StorageBackend,
	artifactRepo interfaces.ArtifactRepository,
	defaultExpiration int,
) *StorageServiceServer {
	return &StorageServiceServer{
		storage:          storage,
		artifactRepo:     artifactRepo,
		defaultExpiration: defaultExpiration,
		logger:           log.WithField("component", "grpc-service"),
	}
}

// CreateArtifact creates an artifact and returns a presigned upload URL
func (s *StorageServiceServer) CreateArtifact(ctx context.Context, req *pb.CreateArtifactRequest) (*pb.CreateArtifactResponse, error) {
	logger := s.logger.WithFields(log.Fields{
		"scan_id": req.ScanId,
		"type":    req.Type,
		"filename": req.Filename,
	})
	logger.Info("Creating artifact")

	// Validate request
	if req.ScanId == "" {
		return nil, status.Error(codes.InvalidArgument, "scan_id is required")
	}
	if req.OrganizationId == "" {
		return nil, status.Error(codes.InvalidArgument, "organization_id is required")
	}
	if req.Filename == "" {
		return nil, status.Error(codes.InvalidArgument, "filename is required")
	}

	// Parse UUIDs
	scanID, err := uuid.Parse(req.ScanId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid scan_id: %v", err)
	}
	orgID, err := uuid.Parse(req.OrganizationId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid organization_id: %v", err)
	}

	// Generate artifact ID - use UUID directly as storage key
	artifactID := uuid.New()
	storagePath := artifactID.String() // Use UUID directly as S3 key

	// Determine expiration
	expiresIn := time.Duration(s.defaultExpiration) * time.Hour
	if req.ExpiresInHours > 0 {
		expiresIn = time.Duration(req.ExpiresInHours) * time.Hour
	}

	// Create artifact domain model
	artifact := &domain.Artifact{
		ID:             artifactID,
		ScanID:         scanID,
		OrganizationID: orgID,
		Type:           convertArtifactTypeFromProto(req.Type),
		Filename:       req.Filename,
		SizeBytes:      req.SizeBytes,
		ContentType:    req.ContentType,
		StoragePath:    storagePath,
		CreatedAt:      time.Now(),
	}

	if expiresIn > 0 {
		expiresAt := time.Now().Add(expiresIn)
		artifact.ExpiresAt = &expiresAt
	}

	// Save artifact metadata to database
	if err := s.artifactRepo.Create(ctx, artifact); err != nil {
		logger.WithError(err).Error("Failed to create artifact in database")
		return nil, status.Errorf(codes.Internal, "failed to create artifact: %v", err)
	}

	// Generate presigned upload URL
	presignedURL, err := s.storage.GeneratePresignedUploadURL(ctx, storagePath, req.ContentType, 24*time.Hour)
	if err != nil {
		logger.WithError(err).Error("Failed to generate presigned upload URL")
		return nil, status.Errorf(codes.Internal, "failed to generate upload URL: %v", err)
	}

	logger.WithField("artifact_id", artifactID.String()).Info("Artifact created successfully")

	return &pb.CreateArtifactResponse{
		Artifact:      convertArtifactToProto(artifact),
		UploadUrl:     presignedURL.URL,
		UploadHeaders: presignedURL.Headers,
	}, nil
}

// GetArtifact retrieves an artifact and returns a presigned download URL
func (s *StorageServiceServer) GetArtifact(ctx context.Context, req *pb.GetArtifactRequest) (*pb.GetArtifactResponse, error) {
	logger := s.logger.WithField("artifact_id", req.Id)
	logger.Debug("Getting artifact")

	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	// Get artifact from database
	artifact, err := s.artifactRepo.Get(ctx, req.Id)
	if err != nil {
		logger.WithError(err).Error("Failed to get artifact from database")
		return nil, status.Errorf(codes.NotFound, "artifact not found: %v", err)
	}

	// Check if artifact is expired
	if artifact.ExpiresAt != nil && artifact.ExpiresAt.Before(time.Now()) {
		return nil, status.Error(codes.FailedPrecondition, "artifact has expired")
	}

	// Determine expiration for download URL
	expiresIn := time.Duration(1) * time.Hour // default 1 hour for downloads
	if req.ExpiresInHours > 0 {
		expiresIn = time.Duration(req.ExpiresInHours) * time.Hour
	}

	// Generate presigned download URL
	presignedURL, err := s.storage.GeneratePresignedDownloadURL(ctx, artifact.StoragePath, expiresIn)
	if err != nil {
		logger.WithError(err).Error("Failed to generate presigned download URL")
		return nil, status.Errorf(codes.Internal, "failed to generate download URL: %v", err)
	}

	logger.Debug("Artifact retrieved successfully")

	return &pb.GetArtifactResponse{
		Artifact:    convertArtifactToProto(artifact),
		DownloadUrl: presignedURL.URL,
	}, nil
}

// DeleteArtifact deletes an artifact
func (s *StorageServiceServer) DeleteArtifact(ctx context.Context, req *pb.DeleteArtifactRequest) (*emptypb.Empty, error) {
	logger := s.logger.WithField("artifact_id", req.Id)
	logger.Info("Deleting artifact")

	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	// Get artifact from database
	artifact, err := s.artifactRepo.Get(ctx, req.Id)
	if err != nil {
		logger.WithError(err).Error("Failed to get artifact from database")
		return nil, status.Errorf(codes.NotFound, "artifact not found: %v", err)
	}

	// Delete from storage backend
	if err := s.storage.DeleteObject(ctx, artifact.StoragePath); err != nil {
		logger.WithError(err).Warn("Failed to delete object from storage (continuing anyway)")
	}

	// Soft delete from database
	if err := s.artifactRepo.Delete(ctx, req.Id); err != nil {
		logger.WithError(err).Error("Failed to delete artifact from database")
		return nil, status.Errorf(codes.Internal, "failed to delete artifact: %v", err)
	}

	logger.Info("Artifact deleted successfully")
	return &emptypb.Empty{}, nil
}

// ListArtifacts lists artifacts with filters
func (s *StorageServiceServer) ListArtifacts(ctx context.Context, req *pb.ListArtifactsRequest) (*pb.ListArtifactsResponse, error) {
	logger := s.logger.WithFields(log.Fields{
		"scan_id": req.ScanId,
		"org_id":  req.OrganizationId,
	})
	logger.Debug("Listing artifacts")

	// Build filter
	filter := interfaces.ArtifactFilter{
		Limit:  int(req.PageSize),
		Offset: 0, // TODO: implement page token-based pagination
	}

	if req.ScanId != "" {
		filter.ScanID = &req.ScanId
	}

	if req.OrganizationId != "" {
		filter.OrganizationID = &req.OrganizationId
	}

	if req.Type != pb.ArtifactType_ARTIFACT_TYPE_UNSPECIFIED {
		artifactType := convertArtifactTypeFromProto(req.Type)
		filter.Type = &artifactType
	}

	// Query database
	artifacts, err := s.artifactRepo.List(ctx, filter)
	if err != nil {
		logger.WithError(err).Error("Failed to list artifacts")
		return nil, status.Errorf(codes.Internal, "failed to list artifacts: %v", err)
	}

	// Convert to proto
	protoArtifacts := make([]*pb.Artifact, len(artifacts))
	for i, artifact := range artifacts {
		protoArtifacts[i] = convertArtifactToProto(artifact)
	}

	return &pb.ListArtifactsResponse{
		Artifacts:  protoArtifacts,
		TotalCount: int32(len(protoArtifacts)),
	}, nil
}

// Multipart upload operations

// InitiateMultipartUpload starts a multipart upload session
func (s *StorageServiceServer) InitiateMultipartUpload(ctx context.Context, req *pb.InitiateMultipartRequest) (*pb.InitiateMultipartResponse, error) {
	logger := s.logger.WithField("artifact_id", req.ArtifactId)
	logger.Info("Initiating multipart upload")

	if req.ArtifactId == "" {
		return nil, status.Error(codes.InvalidArgument, "artifact_id is required")
	}

	// Get artifact from database
	artifact, err := s.artifactRepo.Get(ctx, req.ArtifactId)
	if err != nil {
		logger.WithError(err).Error("Failed to get artifact from database")
		return nil, status.Errorf(codes.NotFound, "artifact not found: %v", err)
	}

	// Initiate multipart upload
	uploadID, err := s.storage.InitiateMultipartUpload(ctx, artifact.StoragePath)
	if err != nil {
		logger.WithError(err).Error("Failed to initiate multipart upload")
		return nil, status.Errorf(codes.Internal, "failed to initiate multipart upload: %v", err)
	}

	logger.WithField("upload_id", uploadID).Info("Multipart upload initiated")
	return &pb.InitiateMultipartResponse{
		UploadId: uploadID,
	}, nil
}

// GetMultipartUploadParts generates presigned URLs for multipart upload parts
func (s *StorageServiceServer) GetMultipartUploadParts(ctx context.Context, req *pb.GetMultipartPartsRequest) (*pb.GetMultipartPartsResponse, error) {
	logger := s.logger.WithFields(log.Fields{
		"artifact_id": req.ArtifactId,
		"upload_id":   req.UploadId,
		"from_part":   req.FromPart,
		"num_parts":   req.NumParts,
	})
	logger.Debug("Getting multipart upload part URLs")

	if req.ArtifactId == "" {
		return nil, status.Error(codes.InvalidArgument, "artifact_id is required")
	}
	if req.UploadId == "" {
		return nil, status.Error(codes.InvalidArgument, "upload_id is required")
	}

	// Get artifact from database
	artifact, err := s.artifactRepo.Get(ctx, req.ArtifactId)
	if err != nil {
		logger.WithError(err).Error("Failed to get artifact from database")
		return nil, status.Errorf(codes.NotFound, "artifact not found: %v", err)
	}

	// Generate presigned URLs for parts
	partURLs, err := s.storage.GetMultipartUploadPartURLs(
		ctx,
		artifact.StoragePath,
		req.UploadId,
		int(req.FromPart),
		int(req.NumParts),
		24*time.Hour,
	)
	if err != nil {
		logger.WithError(err).Error("Failed to generate multipart upload part URLs")
		return nil, status.Errorf(codes.Internal, "failed to generate part URLs: %v", err)
	}

	// Convert to proto
	parts := make([]*pb.UploadPart, len(partURLs))
	for i, partURL := range partURLs {
		parts[i] = &pb.UploadPart{
			PartNumber: int32(i) + req.FromPart,
			Url:        partURL.URL,
			Expiration: partURL.ExpiresAt.Format(time.RFC3339),
		}
	}

	return &pb.GetMultipartPartsResponse{
		Parts: parts,
	}, nil
}

// CompleteMultipartUpload finalizes a multipart upload
func (s *StorageServiceServer) CompleteMultipartUpload(ctx context.Context, req *pb.CompleteMultipartRequest) (*pb.CompleteMultipartResponse, error) {
	logger := s.logger.WithFields(log.Fields{
		"artifact_id": req.ArtifactId,
		"upload_id":   req.UploadId,
		"num_parts":   len(req.Parts),
	})
	logger.Info("Completing multipart upload")

	if req.ArtifactId == "" {
		return nil, status.Error(codes.InvalidArgument, "artifact_id is required")
	}
	if req.UploadId == "" {
		return nil, status.Error(codes.InvalidArgument, "upload_id is required")
	}

	// Get artifact from database
	artifact, err := s.artifactRepo.Get(ctx, req.ArtifactId)
	if err != nil {
		logger.WithError(err).Error("Failed to get artifact from database")
		return nil, status.Errorf(codes.NotFound, "artifact not found: %v", err)
	}

	// Convert parts
	parts := make([]interfaces.CompletedPart, len(req.Parts))
	for i, part := range req.Parts {
		parts[i] = interfaces.CompletedPart{
			PartNumber: int(part.PartNumber),
			ETag:       part.Etag,
		}
	}

	// Complete multipart upload
	err = s.storage.CompleteMultipartUpload(ctx, artifact.StoragePath, req.UploadId, parts)
	if err != nil {
		logger.WithError(err).Error("Failed to complete multipart upload")
		return nil, status.Errorf(codes.Internal, "failed to complete multipart upload: %v", err)
	}

	// Generate download URL
	presignedURL, err := s.storage.GeneratePresignedDownloadURL(ctx, artifact.StoragePath, 1*time.Hour)
	if err != nil {
		logger.WithError(err).Warn("Failed to generate download URL after completion")
	}

	logger.Info("Multipart upload completed successfully")
	return &pb.CompleteMultipartResponse{
		Url:        presignedURL.URL,
		Expiration: presignedURL.ExpiresAt.Format(time.RFC3339),
	}, nil
}

// AbortMultipartUpload cancels a multipart upload
func (s *StorageServiceServer) AbortMultipartUpload(ctx context.Context, req *pb.AbortMultipartRequest) (*emptypb.Empty, error) {
	logger := s.logger.WithFields(log.Fields{
		"artifact_id": req.ArtifactId,
		"upload_id":   req.UploadId,
	})
	logger.Info("Aborting multipart upload")

	if req.ArtifactId == "" {
		return nil, status.Error(codes.InvalidArgument, "artifact_id is required")
	}
	if req.UploadId == "" {
		return nil, status.Error(codes.InvalidArgument, "upload_id is required")
	}

	// Get artifact from database
	artifact, err := s.artifactRepo.Get(ctx, req.ArtifactId)
	if err != nil {
		logger.WithError(err).Error("Failed to get artifact from database")
		return nil, status.Errorf(codes.NotFound, "artifact not found: %v", err)
	}

	// Abort multipart upload
	err = s.storage.AbortMultipartUpload(ctx, artifact.StoragePath, req.UploadId)
	if err != nil {
		logger.WithError(err).Error("Failed to abort multipart upload")
		return nil, status.Errorf(codes.Internal, "failed to abort multipart upload: %v", err)
	}

	logger.Info("Multipart upload aborted successfully")
	return &emptypb.Empty{}, nil
}

// Helper functions

// convertArtifactToProto converts domain.Artifact to proto
func convertArtifactToProto(artifact *domain.Artifact) *pb.Artifact {
	protoArtifact := &pb.Artifact{
		Id:             artifact.ID.String(),
		ScanId:         artifact.ScanID.String(),
		OrganizationId: artifact.OrganizationID.String(),
		Type:           convertArtifactTypeToProto(artifact.Type),
		Filename:       artifact.Filename,
		SizeBytes:      artifact.SizeBytes,
		ContentType:    artifact.ContentType,
		StoragePath:    artifact.StoragePath,
		CreatedAt:      timestamppb.New(artifact.CreatedAt),
	}

	if artifact.ExpiresAt != nil {
		protoArtifact.ExpiresAt = timestamppb.New(*artifact.ExpiresAt)
	}

	return protoArtifact
}

// convertArtifactTypeToProto converts domain.ArtifactType to proto
func convertArtifactTypeToProto(t domain.ArtifactType) pb.ArtifactType {
	switch t {
	case domain.ArtifactTypeSourceCode:
		return pb.ArtifactType_SOURCE_CODE
	case domain.ArtifactTypeScanResults:
		return pb.ArtifactType_SCAN_RESULTS
	case domain.ArtifactTypeReport:
		return pb.ArtifactType_REPORT
	case domain.ArtifactTypeLog:
		return pb.ArtifactType_LOG
	default:
		return pb.ArtifactType_ARTIFACT_TYPE_UNSPECIFIED
	}
}

// convertArtifactTypeFromProto converts proto ArtifactType to domain
func convertArtifactTypeFromProto(t pb.ArtifactType) domain.ArtifactType {
	switch t {
	case pb.ArtifactType_SOURCE_CODE:
		return domain.ArtifactTypeSourceCode
	case pb.ArtifactType_SCAN_RESULTS:
		return domain.ArtifactTypeScanResults
	case pb.ArtifactType_REPORT:
		return domain.ArtifactTypeReport
	case pb.ArtifactType_LOG:
		return domain.ArtifactTypeLog
	default:
		return ""
	}
}